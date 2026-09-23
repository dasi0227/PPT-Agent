import { fetchClient } from './client';

export const slidesApi = {
  render: (id: string, expectedHash: string, signal?: AbortSignal) =>
    fetchClient<string>(`/slides/${encodeURIComponent(id)}/render?expected_hash=${encodeURIComponent(expectedHash)}`, {
      signal, timeoutMs: 30_000, responseType: 'text', headers: { Accept: 'text/html' }, reportError: false,
    }),
};
