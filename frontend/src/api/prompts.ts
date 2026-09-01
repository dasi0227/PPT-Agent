import { fetchClient } from './client';
import type { Prompt, PromptsResponse, PromptWriteRequest } from './types';

export const promptsApi = {
  list: () => fetchClient<PromptsResponse>('/prompts', { reportError: false }),
  get: (id: string) => fetchClient<Prompt>(`/prompts/${encodeURIComponent(id)}`, { reportError: false }),
  create: (request: PromptWriteRequest) => fetchClient<Prompt>('/prompts', {
    method: 'POST',
    body: JSON.stringify(request),
    reportError: false,
  }),
  update: (id: string, request: PromptWriteRequest) => fetchClient<Prompt>(`/prompts/${encodeURIComponent(id)}`, {
    method: 'PUT',
    body: JSON.stringify(request),
    reportError: false,
  }),
  delete: (id: string) => fetchClient<void>(`/prompts/${encodeURIComponent(id)}`, {
    method: 'DELETE',
    reportError: false,
  }),
};
