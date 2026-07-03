import { fetchClient } from './client';
import { Project, Slide, Thread } from './types';

export const projectsApi = {
  list: () => fetchClient<Project[]>('/projects'),
  create: (title: string, theme: string) => fetchClient<Project>('/projects', {
    method: 'POST',
    body: JSON.stringify({ title, theme })
  }),
  get: (id: string) => fetchClient<Project>(`/projects/${id}`),
  getSlides: (id: string) => fetchClient<Slide[]>(`/projects/${id}/slides`),
  getThreads: (id: string) => fetchClient<Thread[]>(`/projects/${id}/threads`),
  createThread: (id: string) => fetchClient<Thread>(`/projects/${id}/threads`, {
    method: 'POST',
    body: JSON.stringify({})
  }),
};
