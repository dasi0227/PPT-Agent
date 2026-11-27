import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { useExportStore } from './exportStore';

const fetchFile = vi.fn<typeof fetch>();
const createObjectURL = vi.fn(() => 'blob:deck');
const revokeObjectURL = vi.fn();
const store = () => useExportStore.getState();

function fileResponse() {
  return new Response('<html><body>Deck</body></html>', {
    headers: {
      'Content-Type': 'text/html;charset=UTF-8',
      'Content-Disposition': 'attachment; filename="deck.html"; filename*=UTF-8\'\'%E6%96%87%E7%A8%BF.html',
    },
  });
}

beforeEach(() => {
  vi.useFakeTimers();
  fetchFile.mockReset();
  createObjectURL.mockClear();
  revokeObjectURL.mockClear();
  vi.stubGlobal('fetch', fetchFile);
  vi.stubGlobal('URL', class extends URL {
    static createObjectURL = createObjectURL;
    static revokeObjectURL = revokeObjectURL;
  });
  useExportStore.setState({ session: null });
});
afterEach(() => {
  vi.runOnlyPendingTimers();
  vi.useRealTimers();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

it('posts once and downloads the response with the UTF-8 filename', async () => {
  const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (this: HTMLAnchorElement) {
    expect(this.download).toBe('文稿.html');
    expect(this.href).toBe('blob:deck');
  });
  fetchFile.mockResolvedValue(fileResponse());

  await store().start('project', 'html');

  expect(fetchFile).toHaveBeenCalledOnce();
  expect(fetchFile).toHaveBeenCalledWith('/api/exports/create', expect.objectContaining({
    method: 'POST', body: JSON.stringify({ project_id: 'project', format: 'html' }),
  }));
  expect(click).toHaveBeenCalledOnce();
  expect(store().session).toBeNull();
  vi.runOnlyPendingTimers();
  expect(revokeObjectURL).toHaveBeenCalledWith('blob:deck');
});

it('keeps a Java error response for retry without downloading it', async () => {
  const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
  fetchFile.mockResolvedValueOnce(new Response(JSON.stringify({ error: { code: 'INVALID_ARGUMENT', message: '幻灯片缺少 HTML' } }), {
    headers: { 'Content-Type': 'application/json' },
  })).mockResolvedValueOnce(fileResponse());

  await store().start('project', 'html');

  expect(click).not.toHaveBeenCalled();
  expect(store().session).toMatchObject({ status: 'error', message: '幻灯片缺少 HTML' });

  await store().retry();
  expect(fetchFile).toHaveBeenCalledTimes(2);
  expect(click).toHaveBeenCalledOnce();
});

it('keeps only one file request in flight', async () => {
  let respond!: (response: Response) => void;
  fetchFile.mockImplementation(() => new Promise(resolve => { respond = resolve; }));
  vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
  const first = store().start('project', 'html');

  await store().start('project', 'pdf');
  expect(store().session).toMatchObject({ status: 'loading', format: 'html' });
  expect(fetchFile).toHaveBeenCalledOnce();

  respond(fileResponse());
  await first;
  expect(store().session).toBeNull();
});
