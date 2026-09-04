import { fetchClient } from './client';
import { CompactContextResponse, ContextWindowSnapshot, Thread } from './types';

export type ThreadHistoryEntry = Record<string, unknown>;

export const threadsApi = {
  list: (projectId: string) => fetchClient<Thread[]>(`/projects/${projectId}/threads`, { reportError: false }),
  create: (projectId: string, title?: string) => fetchClient<Thread>(`/projects/${projectId}/threads`, {
    method: 'POST',
    body: JSON.stringify({ title: title || '' })
  }),
  history: (threadId: string) => fetchClient<ThreadHistoryEntry[]>(`/threads/${threadId}/history`, { reportError: false }),
  contextWindow: (threadId: string, modelProfileName: string) =>
    fetchClient<ContextWindowSnapshot>(
      `/threads/${threadId}/context-window?model_profile_name=${encodeURIComponent(modelProfileName)}`,
      { reportError: false },
    ),
  compact: (threadId: string, modelProfileName: string) =>
    fetchClient<CompactContextResponse>(`/threads/${threadId}/compact`, {
      method: 'POST',
      body: JSON.stringify({ model_profile_name: modelProfileName }),
    }),
  patch: (id: string, patch: {title?: string}) => fetchClient<Thread>(`/threads/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(patch)
  }),
  delete: (id: string) => fetchClient<void>(`/threads/${id}`, {
    method: 'DELETE'
  })
};
