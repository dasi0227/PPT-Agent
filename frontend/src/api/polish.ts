import { fetchClient } from './client';
import type { PolishRequest, PolishResponse } from './types';

export const polishApi = {
  polish: (projectId: string, payload: PolishRequest, signal?: AbortSignal) => (
    fetchClient<PolishResponse>(`/projects/${projectId}/polish`, {
      method: 'POST',
      body: JSON.stringify(payload),
      signal,
      timeoutMs: 15_000,
    })
  ),
};
