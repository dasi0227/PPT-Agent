import { fetchClient } from './client';
import type { Decorations, MutationResponse, PPTMutation, Project, ProjectContentSnapshot, ProjectLogo } from './types';

export const projectsApi = {
  list: () => fetchClient<Project[]>('/projects'),
  create: (topic: string) => fetchClient<Project>('/projects', {
    method: 'POST',
    body: JSON.stringify({ topic })
  }),
  patch: (id: string, patch: {title?: string}) => fetchClient<Project>(`/projects/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(patch)
  }),
  getContent: (id: string) => fetchClient<ProjectContentSnapshot>(`/projects/${id}/content`, { reportError: false }),
  mutate: (id: string, mutation: PPTMutation) => {
    const { expected_scene_revision, ...request } = mutation;
    return fetchClient<MutationResponse>(`/projects/${id}/mutations`, { method: 'POST', body: JSON.stringify(request),
      headers: expected_scene_revision === undefined ? undefined : { 'X-Expected-Scene-Revision': String(expected_scene_revision) } });
  },
  setTheme: (id: string, theme: string) => fetchClient<Project>(`/projects/${id}/theme`, {
    method: 'PATCH',
    body: JSON.stringify({ theme }),
    reportError: false,
  }),
  getDecorations: (id: string, signal?: AbortSignal) => fetchClient<Decorations>(`/projects/${id}/decorations`, {
    signal, reportError: false,
  }),
  setDecorations: (id: string, updates: Partial<Decorations>) => fetchClient<Project>(`/projects/${id}/decorations`, {
    method: 'PATCH', body: JSON.stringify(updates), reportError: false,
  }),
  getLogo: (id: string, signal?: AbortSignal) => fetchClient<ProjectLogo | null>(`/projects/${id}/logo`, {
    signal, reportError: false,
  }),
  uploadLogo: (id: string, file: File) => {
    const body = new FormData();
    body.append('file', file, file.name);
    return fetchClient<Project>(`/projects/${id}/logo`, {
      method: 'PUT', body, reportError: false, timeoutMs: 30_000,
    });
  },
  deleteLogo: (id: string) => fetchClient<Project>(`/projects/${id}/logo`, {
    method: 'DELETE', reportError: false,
  }),
  logoContentUrl: (id: string, hash: string) =>
    `/api/v1/projects/${encodeURIComponent(id)}/logo/content?v=${hash}`,
  delete: (id: string) => fetchClient<void>(`/projects/${id}`, { method: 'DELETE' }),
};
