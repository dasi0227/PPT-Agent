import { fetchClient } from './client';
import { MutationResponse, PPTMutation, Project, ProjectContentSnapshot } from './types';

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
  get: (id: string) => fetchClient<Project>(`/projects/${id}`),
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
  delete: (id: string) => fetchClient<void>(`/projects/${id}`, { method: 'DELETE' }),
};
