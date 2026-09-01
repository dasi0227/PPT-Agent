import { fetchClient } from './client';
import { Thread } from './types';

export type ThreadHistoryEntry = Record<string, unknown>;

export const threadsApi = {
  list: (projectId: string) => fetchClient<Thread[]>(`/projects/${projectId}/threads`, { reportError: false }),
  create: (projectId: string, title?: string) => fetchClient<Thread>(`/projects/${projectId}/threads`, {
    method: 'POST',
    body: JSON.stringify({ title: title || '' })
  }),
  history: (threadId: string) => fetchClient<ThreadHistoryEntry[]>(`/threads/${threadId}/history`, { reportError: false }),
  patch: (id: string, patch: {title?: string}) => fetchClient<Thread>(`/threads/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(patch)
  }),
  delete: (id: string) => fetchClient<void>(`/threads/${id}`, {
    method: 'DELETE'
  })
};
