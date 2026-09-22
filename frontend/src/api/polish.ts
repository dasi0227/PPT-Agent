import { fetchClient } from './client';
import type { PolishRequest, PolishResponse } from './types';

export const polishApi = {
  polish: (projectId: string, payload: PolishRequest, signal?: AbortSignal, onProgress?: (phase: number) => void, commandId?: string) => (
    fetchClient<PolishResponse>(`/projects/${projectId}/polish`, {
      method: 'POST',
      headers: commandId ? { 'X-Command-ID': commandId } : undefined,
      responseType: 'command', onProgress,
      body: JSON.stringify(payload),
      signal,
      timeoutMs: 15_000,
      reportError: false,
    })
  ),
};
