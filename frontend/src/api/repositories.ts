import { fetchClient } from './client';
import type {
  ComponentReference,
  ComponentsResponse,
  Skill,
  ThemesResponse,
  Theme,
} from './types';

export const repositoriesApi = {
  listThemes: () => fetchClient<ThemesResponse>('/themes', { reportError: false }),
  getTheme: (id: string) => fetchClient<Theme>(`/themes/${encodeURIComponent(id)}`, { reportError: false }),
  listComponents: () => fetchClient<ComponentsResponse>('/components', { reportError: false }),
  getComponent: (id: string) => fetchClient<ComponentReference>(`/components/${encodeURIComponent(id)}`, { reportError: false }),
  getSkill: (id: string) => fetchClient<Skill>(`/skills/${encodeURIComponent(id)}`, { reportError: false }),
  setSkillDisabled: (id: string, disabled: boolean) => fetchClient<Skill>(`/skills/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: JSON.stringify({ disabled }),
    reportError: false,
  }),
};
