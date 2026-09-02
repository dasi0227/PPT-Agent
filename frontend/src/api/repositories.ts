import { fetchClient } from './client';
import type {
  ComponentReference,
  ComponentsResponse,
  Skill,
  SkillTag,
  ThemesResponse,
  Theme,
  ThemeTag,
  ComponentTag,
} from './types';

export const repositoriesApi = {
  listThemes: () => fetchClient<ThemesResponse>('/themes', { reportError: false }),
  getTheme: (id: string) => fetchClient<Theme>(`/themes/${encodeURIComponent(id)}`, { reportError: false }),
  updateTheme: (id: string, request: { name: string; description: string; tags: ThemeTag[] }) => fetchClient<Theme>(`/themes/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: JSON.stringify(request),
    reportError: false,
  }),
  deleteTheme: (id: string) => fetchClient<void>(`/themes/${encodeURIComponent(id)}`, {
    method: 'DELETE',
    reportError: false,
  }),
  listComponents: () => fetchClient<ComponentsResponse>('/components', { reportError: false }),
  getComponent: (id: string) => fetchClient<ComponentReference>(`/components/${encodeURIComponent(id)}`, { reportError: false }),
  setComponentDisabled: (id: string, disabled: boolean) => fetchClient<ComponentReference>(`/components/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: JSON.stringify({ disabled }),
    reportError: false,
  }),
  updateComponent: (id: string, request: { name: string; description: string; tags: ComponentTag[] }) => fetchClient<ComponentReference>(`/components/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: JSON.stringify(request),
    reportError: false,
  }),
  deleteComponent: (id: string) => fetchClient<void>(`/components/${encodeURIComponent(id)}`, {
    method: 'DELETE',
    reportError: false,
  }),
  getSkill: (id: string) => fetchClient<Skill>(`/skills/${encodeURIComponent(id)}`, { reportError: false }),
  setSkillDisabled: (id: string, disabled: boolean) => fetchClient<Skill>(`/skills/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: JSON.stringify({ disabled }),
    reportError: false,
  }),
  updateSkill: (id: string, request: { name: string; description: string; tags: SkillTag[] }) => fetchClient<Skill>(`/skills/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: JSON.stringify(request),
    reportError: false,
  }),
  deleteSkill: (id: string) => fetchClient<void>(`/skills/${encodeURIComponent(id)}`, {
    method: 'DELETE',
    reportError: false,
  }),
};
