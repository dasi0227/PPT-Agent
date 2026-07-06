import { fetchClient } from './client';
import { Project, Slide } from './types';

export const projectsApi = {
  list: () => fetchClient<Project[]>('/projects'),
  create: (topic: string, brief: string = '', slide_count: number = 10, language: string = 'zh') => fetchClient<Project>('/projects', {
    method: 'POST',
    body: JSON.stringify({ topic, brief, slide_count, language })
  }),
  get: (id: string) => fetchClient<Project>(`/projects/${id}`),
  getSlides: (id: string) => fetchClient<Slide[]>(`/projects/${id}/slides`),
  delete: (id: string) => fetchClient<void>(`/projects/${id}`, { method: 'DELETE' }),
};
