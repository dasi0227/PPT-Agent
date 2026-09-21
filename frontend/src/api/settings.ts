import { fetchClient } from './client';

export const SIDE_PURPOSES = ['rename', 'compact', 'commit', 'polish', 'handoff', 'kickoff'] as const;
export type SidePurpose = typeof SIDE_PURPOSES[number];
export interface ModelConfig {
  name: string;
  provider: string;
  model: string;
  has_key: boolean;
}
export interface RoadConfig { default: string; fallback: string | null }
export interface ModelSettings {
  revision: string;
  providers: string[];
  llm: ModelConfig[];
  main_road: RoadConfig;
  side_road: RoadConfig & Record<SidePurpose, string | null>;
}
export interface ModelEdit {
  previous_name?: string;
  name: string;
  provider: string;
  model: string;
  key?: string;
}
export type SettingsEdit = Omit<ModelSettings, 'providers' | 'llm'> & { llm: ModelEdit[] };
export const settingsApi = {
  get: () => fetchClient<ModelSettings>('/settings/models', { reportError: false, cache: 'no-store' }),
  save: (settings: SettingsEdit) => fetchClient<ModelSettings>('/settings/models', {
    method: 'PUT', body: JSON.stringify(settings), reportError: false,
  }),
};
