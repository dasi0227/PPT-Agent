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
  renameSlide: (projectId: string, slideId: string, title: string) =>
    fetchClient<ProjectContentSnapshot>(`/projects/${projectId}/slides/${encodeURIComponent(slideId)}`, {
      method: 'PATCH',
      body: JSON.stringify({ title }),
    }),
  addSection: (projectId: string, title?: string) =>
    fetchClient<ProjectContentSnapshot>(`/projects/${projectId}/sections`, {
      method: 'POST',
      body: JSON.stringify(title ? { title } : {}),
    }),
  removeSection: (projectId: string, sectionId: string) =>
    fetchClient<ProjectContentSnapshot>(`/projects/${projectId}/sections/${encodeURIComponent(sectionId)}`, {
      method: 'DELETE',
    }),
  renameSection: (projectId: string, sectionId: string, title: string) =>
    fetchClient<ProjectContentSnapshot>(`/projects/${projectId}/sections/${encodeURIComponent(sectionId)}`, {
      method: 'PATCH',
      body: JSON.stringify({ title }),
    }),
  addSubsection: (projectId: string, sectionId: string, title?: string) =>
    fetchClient<ProjectContentSnapshot>(`/projects/${projectId}/sections/${encodeURIComponent(sectionId)}/subsections`, {
      method: 'POST',
      body: JSON.stringify(title ? { title } : {}),
    }),
  removeSubsection: (projectId: string, sectionId: string, subsectionId: string) =>
    fetchClient<ProjectContentSnapshot>(
      `/projects/${projectId}/sections/${encodeURIComponent(sectionId)}/subsections/${encodeURIComponent(subsectionId)}`,
      { method: 'DELETE' },
    ),
  renameSubsection: (projectId: string, sectionId: string, subsectionId: string, title: string) =>
    fetchClient<ProjectContentSnapshot>(
      `/projects/${projectId}/sections/${encodeURIComponent(sectionId)}/subsections/${encodeURIComponent(subsectionId)}`,
      { method: 'PATCH', body: JSON.stringify({ title }) },
    ),
};
