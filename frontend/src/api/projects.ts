import { fetchClient } from './client';
import { MutationResponse, PPTMutation, Project, ProjectContentSnapshot } from './types';

export const projectsApi = {
  list: () => fetchClient<Project[]>('/projects'),
  create: (topic: string, brief: string = '', slide_count: number = 10, language: string = 'zh') => fetchClient<Project>('/projects', {
    method: 'POST',
    body: JSON.stringify({ topic, brief, slide_count, language })
  }),
  patch: (id: string, patch: {title?: string}) => fetchClient<Project>(`/projects/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(patch)
  }),
  get: (id: string) => fetchClient<Project>(`/projects/${id}`),
  getContent: (id: string) => fetchClient<ProjectContentSnapshot>(`/projects/${id}/content`),
  mutate: (id: string, mutation: PPTMutation) => fetchClient<MutationResponse>(`/projects/${id}/mutations`, { method: 'POST', body: JSON.stringify(mutation) }),
  setTheme: (id: string, theme: string) => fetchClient<Project>(`/projects/${id}/theme`, {
    method: 'PATCH',
    body: JSON.stringify({ theme }),
  }),
  delete: (id: string) => fetchClient<void>(`/projects/${id}`, { method: 'DELETE' }),
};
