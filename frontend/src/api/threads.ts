import { fetchClient } from './client';
import type { DeletionResponse } from './deletions';
import { parseCommitReceipt } from './session';
import { runCommand } from './commands';
import { CompactContextResponse, ContextWindowSnapshot, Thread, ThreadNamingAction } from './types';

async function renameThread(id: string, input: { mode: 'automatic' | 'manual'; title?: string },
  options: { signal?: AbortSignal; onProgress?: (phase: number) => void; commandId?: string }): Promise<Thread> {
  await runCommand<{ title: string }>(id, 'rename', input, options);
  // A command result is not a second copy of thread metadata. Read the current
  // row after completion so title and naming revision have the same authority.
  return fetchClient<Thread>(`/threads/${encodeURIComponent(id)}`, { signal: options.signal, reportError: false });
}

export const threadsApi = {
  list: (projectId: string) => fetchClient<Thread[]>(`/projects/${projectId}/threads`, { reportError: false }),
  create: (projectId: string, title?: string) => fetchClient<Thread>(`/projects/${projectId}/threads`, {
    method: 'POST',
    body: JSON.stringify({ title: title || '' })
  }),
  clearPlan: (id: string, sceneRevision: string, planRevision: string) => fetchClient<import('./session').CommitReceipt>(`/threads/${id}/plan`, { method: 'DELETE', body: JSON.stringify({ scene_revision: sceneRevision, plan_revision: planRevision, request_key: crypto.randomUUID() }) }).then(parseCommitReceipt),
  contextWindow: (threadId: string, modelProfileName: string, sceneRevision: string, signal?: AbortSignal) =>
    fetchClient<ContextWindowSnapshot>(
      `/threads/${threadId}/context-window?model_profile_name=${encodeURIComponent(modelProfileName)}&scene_revision=${encodeURIComponent(sceneRevision)}`,
      { reportError: false, signal },
    ),
  compact: (threadId: string, signal?: AbortSignal, onProgress?: (phase: number) => void, commandId?: string) => runCommand<CompactContextResponse>(threadId, 'compact', {}, { signal, onProgress, commandId }),
  generateName: (id: string, signal?: AbortSignal, onProgress?: (phase: number) => void, commandId?: string) => renameThread(id, { mode: 'automatic' }, { signal, onProgress, commandId }),
  patch: (id: string, patch: {title?: string}, commandId?: string, signal?: AbortSignal) => renameThread(id, { mode: 'manual', title: patch.title }, { signal, commandId }),
  suggest: (id: string, commandId: string) => runCommand<{ requests: string[] }>(id, 'suggest', {}, { commandId }),
  setAutoSuggest: (id: string, enabled: boolean) => fetchClient<Thread>(`/threads/${id}`, { method: 'PATCH', reportError: false, body: JSON.stringify({ auto_suggest_enabled: enabled }) }),
  naming: async (id: string, operationId: string, action: ThreadNamingAction, title?: string): Promise<Thread> => {
    if (action === 'enable' || action === 'disable') return fetchClient<Thread>(`/threads/${id}`, { method: 'PATCH', body: JSON.stringify({ auto_rename_enabled: action === 'enable' }) });
    return renameThread(id, { mode: action === 'manual' ? 'manual' : 'automatic', title }, { commandId: `rename:${operationId}` });
  },
  delete: (id: string) => fetchClient<DeletionResponse>(`/threads/${id}`, {
    method: 'DELETE'
  })
};
