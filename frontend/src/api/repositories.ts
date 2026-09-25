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
  themeExample: (name: string) => fetchClient<{ html: string }>(`/runtime/theme-examples/${encodeURIComponent(name)}`, { reportError: false }),
  listThemes: () => fetchClient<ThemesResponse>('/themes', { reportError: false }),
  getTheme: (id: string) => fetchClient<Theme>(`/themes/${encodeURIComponent(id)}`, { reportError: false }),
  setThemeDisabled: (id: string, disabled: boolean) => fetchClient<void>(`/resources/theme/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: JSON.stringify({ disabled }),
    reportError: false,
  }).then(() => fetchClient<Theme>(`/themes/${encodeURIComponent(id)}`, { reportError: false })),
  updateTheme: (id: string, request: { name: string; description: string; tags: ThemeTag[] }) => fetchClient<void>(`/resources/theme/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: JSON.stringify(request),
    reportError: false,
  }).then(() => fetchClient<Theme>(`/themes/${encodeURIComponent(id)}`, { reportError: false })),
  deleteTheme: (id: string) => fetchClient<void>(`/resources/theme/${encodeURIComponent(id)}`, {
    method: 'DELETE',
    reportError: false,
  }),
  listComponents: () => fetchClient<ComponentsResponse>('/components', { reportError: false }),
  getComponent: (id: string) => fetchClient<ComponentReference>(`/components/${encodeURIComponent(id)}`, { reportError: false }),
  setComponentDisabled: (id: string, disabled: boolean) => fetchClient<void>(`/resources/component/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: JSON.stringify({ disabled }),
    reportError: false,
  }).then(() => fetchClient<ComponentReference>(`/components/${encodeURIComponent(id)}`, { reportError: false })),
  updateComponent: (id: string, request: { name: string; description: string; tags: ComponentTag[] }) => fetchClient<void>(`/resources/component/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: JSON.stringify(request),
    reportError: false,
  }).then(() => fetchClient<ComponentReference>(`/components/${encodeURIComponent(id)}`, { reportError: false })),
  deleteComponent: (id: string) => fetchClient<void>(`/resources/component/${encodeURIComponent(id)}`, {
    method: 'DELETE',
    reportError: false,
  }),
  getSkill: (id: string) => fetchClient<Skill>(`/skills/${encodeURIComponent(id)}`, { reportError: false }),
  setSkillDisabled: (id: string, disabled: boolean) => fetchClient<void>(`/resources/skill/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: JSON.stringify({ disabled }),
    reportError: false,
  }).then(() => fetchClient<Skill>(`/skills/${encodeURIComponent(id)}`, { reportError: false })),
  updateSkill: (id: string, request: { name: string; description: string; tags: SkillTag[] }) => fetchClient<void>(`/resources/skill/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: JSON.stringify(request),
    reportError: false,
  }).then(() => fetchClient<Skill>(`/skills/${encodeURIComponent(id)}`, { reportError: false })),
  deleteSkill: (id: string) => fetchClient<void>(`/resources/skill/${encodeURIComponent(id)}`, {
    method: 'DELETE',
    reportError: false,
  }),
};
