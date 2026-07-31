import { fetchClient } from './client';
import { Slide } from './types';

export interface SlidePatch {
  title?: string;
  subtitle?: string;
  bullets?: string[];
  content_intent?: string;
  chart_intent?: { type: string; data_hint?: string };
  layout?: string;
  steps?: number;
}

export const slidesApi = {
  get: (id: string) => fetchClient<Slide>(`/slides/${id}`),
  render: async (id: string, signal?: AbortSignal) => {
    const response = await fetch(`/api/v1/slides/${encodeURIComponent(id)}/render`, { signal });
    if (!response.ok) {
      throw new Error(`slide render failed: ${response.status}`);
    }
    return response.text();
  },
  patch: (id: string, patch: SlidePatch) =>
    fetchClient<Slide>(`/slides/${id}`, { method: 'PATCH', body: JSON.stringify(patch) }),
  add: (projectId: string, opts: { after_slide_id?: string; layout?: string } = {}) =>
    fetchClient<Slide>(`/projects/${projectId}/slides`, { method: 'POST', body: JSON.stringify(opts) }),
  remove: (id: string) => fetchClient<void>(`/slides/${id}`, { method: 'DELETE' }),
  reorder: (projectId: string, orderedIds: string[]) =>
    fetchClient<void>(`/projects/${projectId}/slides/reorder`, { method: 'POST', body: JSON.stringify({ ordered_ids: orderedIds }) }),
};
