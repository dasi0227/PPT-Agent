import { fetchClient } from './client';
import { CompactContextResponse, ContextWindowSnapshot, Thread, ThreadNamingAction, ThreadNamingResponse } from './types';

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
  compact: (threadId: string, signal?: AbortSignal, onProgress?: (phase: number) => void, commandId?: string) =>
    fetchClient<CompactContextResponse>(`/threads/${threadId}/compact`, {
      headers: commandId ? { 'X-Command-ID': commandId } : undefined, signal, onProgress, responseType: 'command', reportError: false, timeoutMs: 60_000,
      method: 'POST',
      body: JSON.stringify({}),
    }),
  generateName: (id: string, signal: AbortSignal, onProgress: (phase: number) => void, commandId?: string) => fetchClient<Thread>(`/threads/${id}/rename`, { method: 'POST', body: '{}', headers: commandId ? { 'X-Command-ID': commandId } : undefined, signal, onProgress, responseType: 'command', reportError: false, timeoutMs: 25_000 }),
  patch: (id: string, patch: {title?: string}, commandId?: string, signal?: AbortSignal) => fetchClient<Thread>(`/threads/${id}`, {
    method: 'PATCH',
    headers: commandId ? { 'X-Command-ID': commandId } : undefined, signal,
    body: JSON.stringify(patch)
  }),
	naming: (id: string, operationId: string, action: ThreadNamingAction, title?: string) =>
		fetchClient<ThreadNamingResponse>(`/threads/${id}/naming`, {
			method: 'POST',
      headers: { 'X-Command-ID': `rename:${operationId}` },
			body: JSON.stringify({ operation_id: operationId, action, ...(title !== undefined ? { title } : {}) }),
		}),
  delete: (id: string) => fetchClient<void>(`/threads/${id}`, {
    method: 'DELETE'
  })
};
