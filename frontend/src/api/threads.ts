import { fetchClient } from './client';
import { runCommand } from './commands';
import { loadThreadHistory } from './threadJournal';
import { CompactContextResponse, ContextWindowSnapshot, Thread, ThreadNamingAction } from './types';

export const threadsApi = {
  list: (projectId: string) => fetchClient<Thread[]>(`/projects/${projectId}/threads`, { reportError: false }),
  create: (projectId: string, title?: string) => fetchClient<Thread>(`/projects/${projectId}/threads`, {
    method: 'POST',
    body: JSON.stringify({ title: title || '' })
  }),
  history: async (threadId: string) => (await loadThreadHistory(threadId)).events,
  contextWindow: (threadId: string, modelProfileName: string) =>
    fetchClient<ContextWindowSnapshot>(
      `/threads/${threadId}/context-window?model_profile_name=${encodeURIComponent(modelProfileName)}`,
      { reportError: false },
    ),
  compact: (threadId: string, signal?: AbortSignal, onProgress?: (phase: number) => void, commandId?: string) => runCommand<CompactContextResponse>(threadId, 'compact', {}, { signal, onProgress, commandId }),
  generateName: (id: string, signal?: AbortSignal, onProgress?: (phase: number) => void, commandId?: string) => runCommand<Thread>(id, 'rename', { mode: 'automatic' }, { signal, onProgress, commandId }),
  patch: (id: string, patch: {title?: string}, commandId?: string, signal?: AbortSignal) => runCommand<Thread>(id, 'rename', { mode: 'manual', title: patch.title }, { signal, commandId }),
  suggest: (id: string) => runCommand<{ requests: string[] }>(id, 'suggest', {}),
  setAutoSuggest: (id: string, enabled: boolean) => fetchClient<Thread>(`/threads/${id}`, { method: 'PATCH', reportError: false, body: JSON.stringify({ auto_suggest_enabled: enabled }) }),
  naming: async (id: string, operationId: string, action: ThreadNamingAction, title?: string): Promise<Thread> => {
    if (action === 'enable' || action === 'disable') return fetchClient<Thread>(`/threads/${id}`, { method: 'PATCH', body: JSON.stringify({ auto_rename_enabled: action === 'enable' }) });
    return runCommand<Thread>(id, 'rename', { mode: action === 'manual' ? 'manual' : 'automatic', title }, { commandId: `rename:${operationId}` });
  },
  delete: (id: string) => fetchClient<void>(`/threads/${id}`, {
    method: 'DELETE'
  })
};
