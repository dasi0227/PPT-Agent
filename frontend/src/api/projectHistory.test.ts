import { afterEach, describe, expect, it, vi } from 'vitest';
import { fetchClient, RequestCanceledError } from './client';
import { useHistoryConfirmationStore } from '../stores/historyConfirmationStore';

const requireConfirmation = () => new Response(JSON.stringify({ error: { code: 'HISTORY_CONFIRM_REQUIRED', message: '丢弃后续历史？', details: { revision: '8' } } }), { status: 409 });
afterEach(() => { vi.restoreAllMocks(); useHistoryConfirmationStore.setState({ pending: null }); });
describe('project history request boundary', () => {
  it('waits for confirmation and binds retry to the preview revision', async () => {
    const fetch = vi.fn().mockResolvedValueOnce(requireConfirmation()).mockResolvedValueOnce(new Response('{"accepted":true}'));
    vi.stubGlobal('fetch', fetch);
    const request = fetchClient('/projects/p/threads', { method: 'POST', body: '{"title":"new"}' });
    await vi.waitFor(() => expect(useHistoryConfirmationStore.getState().pending).not.toBeNull());
    expect(fetch).toHaveBeenCalledTimes(1);
    useHistoryConfirmationStore.getState().pending!.resolve(true);
    await expect(request).resolves.toEqual({ accepted: true });
    expect(new Headers(fetch.mock.calls[1][1].headers).get('X-Discard-Future-Revision')).toBe('8');
    expect(fetch.mock.calls[1][1].body).toBe('{"title":"new"}');
  });
  it('canceling leaves the write unexecuted', async () => {
    const fetch = vi.fn().mockResolvedValue(requireConfirmation()); vi.stubGlobal('fetch', fetch);
    const request = fetchClient('/projects/p/attachments', { method: 'POST', body: new FormData() }).catch((error: unknown) => error);
    await vi.waitFor(() => expect(useHistoryConfirmationStore.getState().pending).not.toBeNull());
    useHistoryConfirmationStore.getState().pending!.resolve(false);
    expect(await request).toBeInstanceOf(RequestCanceledError);
    expect(fetch).toHaveBeenCalledTimes(1);
  });
});
