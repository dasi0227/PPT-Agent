import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { HistoryBanner, ProjectHistoryDialogs, RollbackButton } from './ProjectHistoryControls';
import { MessageMetaActions } from './MessageMetaActions';
import { useProjectHistoryStore } from '../../stores/projectHistoryStore';
import { useProjectStore } from '../../stores/projectStore';
import { projectHistoryApi } from '../../api/projectHistory';

beforeEach(() => {
  useProjectStore.setState({ activeProjectId: 'p' });
  useProjectHistoryStore.setState({ states: { p: { revision: 1, scene_revision: 0, latest: 'live', checkpoints: [{ run_id: 'r', thread_id: 't', time: 1000, sequence: 1 }] } }, dialog: null, busy: false, error: null });
  vi.spyOn(projectHistoryApi, 'state').mockResolvedValue(useProjectHistoryStore.getState().states.p);
});
afterEach(() => vi.restoreAllMocks());
describe('checkpoint controls', () => {
  it('places rollback after copy, with no separate steering checkpoint', () => {
    const view = render(<MessageMetaActions text="hello" timestamp={1000} label="复制"><RollbackButton runId="r" /></MessageMetaActions>);
    expect(screen.getAllByRole('button').map((button) => button.getAttribute('aria-label'))).toEqual(['复制', '回退到此消息发送前']);
    view.rerender(<RollbackButton runId="r" steering />);
    expect(screen.queryByRole('button')).toBeNull();
  });
  it('previews before writing and cancellation sends no switch', async () => {
    const preview = vi.spyOn(projectHistoryApi, 'preview').mockResolvedValue({ revision: 1, time: 1000, input: 'hello', runs: 3 });
    const execute = vi.spyOn(projectHistoryApi, 'switch');
    render(<><HistoryBanner /><ProjectHistoryDialogs /></>);
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: '恢复到最新' })); });
    expect(preview).toHaveBeenCalledWith('p', undefined);
    expect(screen.getByText('恢复到最新现场？')).toBeInTheDocument();
    expect(execute).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: '取消' }));
    expect(execute).not.toHaveBeenCalled();
  });

  it('closes the history preview with Escape when no operation is running', async () => {
    vi.spyOn(projectHistoryApi, 'preview').mockResolvedValue({ revision: 1, time: 1000, input: 'hello', runs: 3 });
    render(<><HistoryBanner /><ProjectHistoryDialogs /></>);
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: '恢复到最新' })); });

    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' });

    await waitFor(() => expect(screen.queryByText('恢复到最新现场？')).not.toBeInTheDocument());
  });
});
