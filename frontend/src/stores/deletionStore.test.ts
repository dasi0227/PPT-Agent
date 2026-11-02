import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { PendingDeletion } from '../api/deletions';

const mocks = vi.hoisted(() => ({
  list: vi.fn(), undo: vi.fn(), listThreads: vi.fn(), loadProjects: vi.fn(), openProject: vi.fn(),
  loadThreads: vi.fn(), setActiveThread: vi.fn(),
}));
vi.mock('../api/deletions', async (original) => ({
  ...await original<typeof import('../api/deletions')>(),
  deletionsApi: { list: mocks.list, undo: mocks.undo },
}));
vi.mock('../api/threads', () => ({ threadsApi: { list: mocks.listThreads } }));
vi.mock('./projectStore', () => ({ useProjectStore: { getState: () => ({ loadProjects: mocks.loadProjects, openProject: mocks.openProject }) } }));
vi.mock('./threadStore', () => ({ useThreadStore: { getState: () => ({ loadThreads: mocks.loadThreads, setActiveThread: mocks.setActiveThread }) } }));

import { deletionNotice } from '../api/deletions';
import { isProjectDeleted, isThreadDeleted, markProjectDeleted, markThreadDeleted } from './deletedTargets';
import { rememberDeletion, recoverDeletionNotices, useDeletionStore } from './deletionStore';
import { useToastStore } from './toastStore';

function target(kind: PendingDeletion['kind'], token: string): PendingDeletion {
  return { kind, id: kind === 'project' ? 'p' : 't', project_id: 'p', token, title: '', undo_until: Date.now() + 5_000 };
}

beforeEach(async () => {
  vi.clearAllMocks();
  mocks.list.mockResolvedValue({ deletions: [], server_now: Date.now() });
  mocks.undo.mockResolvedValue(undefined);
  mocks.listThreads.mockResolvedValue([{ id: 't' }]);
  mocks.loadProjects.mockResolvedValue(undefined);
  mocks.loadThreads.mockResolvedValue(undefined);
  await recoverDeletionNotices();
  useToastStore.getState().clearToasts();
  useDeletionStore.setState({ restored: null });
});

describe('deletion undo feedback', () => {
  it.each(['project', 'thread'] as const)('restores %s content and opens the restored target', async (kind) => {
    const deletion = target(kind, `undo-${kind}`);
    markProjectDeleted('p'); markThreadDeleted('t');
    rememberDeletion({ deletion, server_now: Date.now() }, Date.now());
    const toast = useToastStore.getState().toasts[0];
    expect(toast.message).toBe(kind === 'project' ? '项目已删除' : '会话已删除');
    toast.action?.onClick();
    toast.action?.onClick();
    await vi.waitFor(() => expect(useDeletionStore.getState().restored?.token).toBe(deletion.token));
    expect(mocks.undo).toHaveBeenCalledTimes(1);
    expect(isProjectDeleted('p')).toBe(false);
    expect(isThreadDeleted('t')).toBe(false);
    expect(mocks.loadProjects).toHaveBeenCalled();
    expect(mocks.openProject).toHaveBeenCalledWith('p');
    expect(mocks.loadThreads).toHaveBeenCalledWith('p');
    if (kind === 'thread') expect(mocks.setActiveThread).toHaveBeenCalledWith('p', 't');
  });

  it('recovers only the remaining window and preserves manual dismissal', async () => {
    const deletion = target('project', 'recovered');
    deletion.undo_until = Date.now() + 1_500;
    mocks.list.mockImplementation(async () => ({ deletions: [deletion], server_now: Date.now() }));
    await recoverDeletionNotices();
    const first = useToastStore.getState().toasts[0];
    expect(first.expiresAt! - Date.now()).toBeLessThanOrEqual(1_500);
    await recoverDeletionNotices();
    expect(useToastStore.getState().toasts).toHaveLength(1);
    expect(useToastStore.getState().toasts[0].expiresAt).toBe(first.expiresAt);
    useToastStore.getState().removeToast(first.id);
    await recoverDeletionNotices();
    expect(useToastStore.getState().toasts).toHaveLength(0);
  });

  it('does not let an older recovery response remove a newly accepted deletion', async () => {
    let finish!: (response: { deletions: PendingDeletion[]; server_now: number }) => void;
    mocks.list.mockReturnValueOnce(new Promise((resolve) => { finish = resolve; }));
    const refresh = recoverDeletionNotices();
    const deletion = target('thread', 'newer');
    rememberDeletion({ deletion, server_now: Date.now() }, Date.now());
    finish({ deletions: [], server_now: Date.now() });
    await refresh;
    expect(useToastStore.getState().toasts).toHaveLength(1);
  });

  it('deducts network time while translating server deadlines to the local clock', () => {
    expect(deletionNotice({ ...target('project', 'clock'), undo_until: 105_000 }, 100_000, 10_000).expiresAt).toBe(15_000);
  });
});
