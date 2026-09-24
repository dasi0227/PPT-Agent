import { fetchClient } from './client';

export const SIDE_PURPOSES = ['rename', 'compact', 'commit', 'polish', 'handoff', 'kickoff'] as const;
export type SidePurpose = typeof SIDE_PURPOSES[number];
export type ModelProtocol = 'responses' | 'anthropic';
export interface ModelConfig {
  name: string;
  /** Read-only brand inferred by the backend from model. */
  readonly provider: string;
  protocol: ModelProtocol;
  base_url: string;
  model: string;
  has_key: boolean;
}
export interface RoadConfig { default: string; fallback: string | null }
export interface ModelSettings {
  revision: string;
  protocols: ModelProtocol[];
  llm: ModelConfig[];
  main_road: RoadConfig;
  side_road: RoadConfig & Record<SidePurpose, string | null>;
}
export interface ModelEdit {
  previous_name?: string;
  name: string;
  protocol: ModelProtocol;
  base_url: string;
  model: string;
  key?: string;
}
export type SettingsEdit = Omit<ModelSettings, 'protocols' | 'llm'> & { llm: ModelEdit[] };
export const settingsApi = {
  get: () => fetchClient<ModelSettings>('/settings/models', { reportError: false, cache: 'no-store' }),
  reload: () => fetchClient<ModelSettings>('/settings/models/reload', {
    method: 'POST', reportError: false, cache: 'no-store',
  }),
  save: (settings: SettingsEdit) => fetchClient<ModelSettings>('/settings/models', {
    method: 'PUT', body: JSON.stringify(settings), reportError: false,
  }),
};
