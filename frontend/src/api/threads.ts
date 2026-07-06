import { fetchClient } from './client';
import { Thread } from './types';

export type ThreadHistoryEntry = Record<string, unknown>;

export const threadsApi = {
  list: (projectId: string) => fetchClient<Thread[]>(`/projects/${projectId}/threads`),
  create: (projectId: string, title?: string) =>
    fetchClient<Thread>(`/projects/${projectId}/threads`, {
      method: 'POST',
      body: JSON.stringify(title ? { title } : {}),
    }),
  history: (threadId: string) => fetchClient<ThreadHistoryEntry[]>(`/threads/${threadId}/history`),
  delete: (threadId: string) => fetchClient<void>(`/threads/${threadId}`, { method: 'DELETE' }),
};
