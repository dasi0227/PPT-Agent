import { fetchClient } from './client';
import { ProjectContentSnapshot, Slide } from './types';

export interface SlidePlacement {
  slide_id: string;
  section_id: string;
  subsection_id?: string;
}

export const slidesApi = {
  render: (id: string, signal?: AbortSignal) =>
    fetchClient<string>(`/slides/${encodeURIComponent(id)}/render`, {
      signal,
      timeoutMs: 30_000,
      responseType: 'text',
      headers: { Accept: 'text/html' },
    }),
  add: (projectId: string, opts: { after_slide_id?: string; layout?: string } = {}) =>
    fetchClient<Slide>(`/projects/${projectId}/slides`, { method: 'POST', body: JSON.stringify(opts) }),
  remove: (id: string) => fetchClient<void>(`/slides/${id}`, { method: 'DELETE' }),
  reorder: (projectId: string, orderedIds: string[]) =>
    fetchClient<void>(`/projects/${projectId}/slides/reorder`, { method: 'POST', body: JSON.stringify({ ordered_ids: orderedIds }) }),
  restructure: (projectId: string, orderedIds: string[], placements: SlidePlacement[]) =>
    fetchClient<ProjectContentSnapshot>(`/projects/${projectId}/slides/restructure`, {
      method: 'POST',
      body: JSON.stringify({ ordered_ids: orderedIds, placements }),
    }),
};
