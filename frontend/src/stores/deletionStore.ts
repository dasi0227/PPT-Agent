import { create } from 'zustand';
import { APIError } from '../api/client';
import { deletionNotice, deletionsApi, type DeletionNotice, type DeletionResponse } from '../api/deletions';
import { threadsApi } from '../api/threads';
import { restoreProject, restoreThread } from './deletedTargets';
import { useProjectStore } from './projectStore';
import { useThreadStore } from './threadStore';
import { showGlobalError, useToastStore } from './toastStore';

interface DeletionState {
  restored: { projectId: string; threadId?: string; token: string } | null;
}

export const useDeletionStore = create<DeletionState>(() => ({ restored: null }));

const notices = new Map<string, { deletion: DeletionNotice; toastId: string }>();
const restoring = new Set<string>();
let refreshVersion = 0;

function removeNotice(token: string) {
  const notice = notices.get(token);
  if (notice) useToastStore.getState().removeToast(notice.toastId);
  notices.delete(token);
}

function showDeletion(deletion: DeletionNotice) {
  for (const [token, notice] of notices) if (notice.deletion.expiresAt <= Date.now()) removeNotice(token);
  if (deletion.expiresAt <= Date.now() || restoring.has(deletion.token)) return;
  if (notices.has(deletion.token)) return;
  // A project deletion temporarily takes precedence over its pending threads.
  if (deletion.kind === 'project') {
    for (const [token, notice] of notices) {
      if (notice.deletion.kind === 'thread' && notice.deletion.project_id === deletion.project_id) removeNotice(token);
    }
  }
  const toastId = useToastStore.getState().pushToast(
    deletion.kind === 'project' ? '项目已删除' : '会话已删除',
    'success',
    { label: '撤销', onClick: () => { void undoDeletion(deletion); } },
    { expiresAt: deletion.expiresAt },
  );
  notices.set(deletion.token, { deletion, toastId });
}

export function rememberDeletion(response: DeletionResponse, startedAt: number) {
  refreshVersion += 1;
  showDeletion(deletionNotice(response.deletion, response.server_now, startedAt));
}

export async function recoverDeletionNotices() {
  const version = ++refreshVersion;
  const startedAt = Date.now();
  const response = await deletionsApi.list();
  if (version !== refreshVersion) return;
  const current = new Set(response.deletions.map((deletion) => deletion.token));
  for (const token of notices.keys()) if (!current.has(token)) removeNotice(token);
  for (const deletion of response.deletions) showDeletion(deletionNotice(deletion, response.server_now, startedAt));
}

async function undoDeletion(deletion: DeletionNotice) {
  if (restoring.has(deletion.token)) return;
  restoring.add(deletion.token);
  removeNotice(deletion.token);
  refreshVersion += 1;
  let succeeded = false;
  try {
    await deletionsApi.undo(deletion);
    restoreProject(deletion.project_id);
    if (deletion.kind === 'thread') restoreThread(deletion.id);
    // Use the current backend inventory so independently deleted threads retain
    // their fences when a parent project is restored.
    const threads = await threadsApi.list(deletion.project_id);
    threads.forEach((thread) => restoreThread(thread.id));
    await useProjectStore.getState().loadProjects();
    useProjectStore.getState().openProject(deletion.project_id);
    await useThreadStore.getState().loadThreads(deletion.project_id);
    if (deletion.kind === 'thread') useThreadStore.getState().setActiveThread(deletion.project_id, deletion.id);
    useDeletionStore.setState({ restored: {
      projectId: deletion.project_id, threadId: deletion.kind === 'thread' ? deletion.id : undefined, token: deletion.token,
    } });
    succeeded = true;
  } catch (error) {
    const message = error instanceof Error ? error.message : '撤销失败，请重试';
    restoring.delete(deletion.token);
    if (!(error instanceof APIError && error.status === 409) && deletion.expiresAt > Date.now()) {
      const toastId = useToastStore.getState().pushToast(message, 'error', {
        label: '撤销', onClick: () => { void undoDeletion(deletion); },
      }, { expiresAt: deletion.expiresAt });
      notices.set(deletion.token, { deletion, toastId });
    } else showGlobalError(message);
  } finally {
    restoring.delete(deletion.token);
    if (succeeded) void recoverDeletionNotices().catch(() => {});
  }
}
