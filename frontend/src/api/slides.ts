import { fetchClient } from './client';

export const slidesApi = {
  render: (id: string, signal?: AbortSignal) =>
    fetchClient<string>(`/slides/${encodeURIComponent(id)}/render`, {
      signal, timeoutMs: 30_000, responseType: 'text', headers: { Accept: 'text/html' },
    }),
};
