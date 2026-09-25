import { fetchClient } from './client';
import type { Snippet, SnippetsResponse, SnippetWriteRequest } from './types';

const get = (id: string) => fetchClient<Snippet>(`/snippets/${encodeURIComponent(id)}`, { reportError: false });
export const snippetsApi = {
  list: () => fetchClient<SnippetsResponse>('/snippets', { reportError: false }),
  get,
  create: (request: SnippetWriteRequest) => fetchClient<Snippet>('/snippets', {
    method: 'POST', body: JSON.stringify(request), reportError: false,
  }),
  update: (id: string, request: Pick<SnippetWriteRequest, 'name' | 'description' | 'tags'>) => fetchClient<void>(`/resources/snippet/${encodeURIComponent(id)}`, {
    method: 'PATCH', body: JSON.stringify(request), reportError: false,
  }).then(() => get(id)),
  writeContent: (id: string, content: string) => fetchClient<Snippet>(`/snippets/${encodeURIComponent(id)}/content`, {
    method: 'PUT', body: JSON.stringify({ content }), reportError: false,
  }),
  setDisabled: (id: string, disabled: boolean) => fetchClient<void>(`/resources/snippet/${encodeURIComponent(id)}`, {
    method: 'PATCH', body: JSON.stringify({ disabled }), reportError: false,
  }).then(() => get(id)),
  delete: (id: string) => fetchClient<void>(`/resources/snippet/${encodeURIComponent(id)}`, {
    method: 'DELETE', reportError: false,
  }),
};
