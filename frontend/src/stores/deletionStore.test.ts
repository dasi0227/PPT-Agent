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
});
