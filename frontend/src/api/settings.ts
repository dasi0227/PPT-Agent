import { fetchClient, postJson } from './client';

export const SIDE_PURPOSES = ['rename', 'compact', 'commit', 'polish', 'handoff', 'suggest'] as const;
export type SidePurpose = typeof SIDE_PURPOSES[number];
export type Route = 'main_default' | 'main_fallback' | 'side_default' | 'side_fallback' | SidePurpose;
export type ModelProtocol = 'responses' | 'anthropic';
export const providers = ['deepseek', 'kimi', 'mimo', 'gpt', 'claude', 'gemini', 'qwen', 'minimax', 'glm', 'grok'] as const;
export type Provider = typeof providers[number];
export interface ModelConfig {
  id: string;
  name: string;
  protocol: ModelProtocol;
  provider: Provider;
  base_url: string;
  model: string;
}
export interface ModelSettings {
  models: ModelConfig[];
  routes: Record<Route, string | null>;
}
type ModelInput = Omit<ModelConfig, 'id'> & { key: string };
type ModelUpdate = { id: string } & { [Field in keyof ModelInput]?: ModelInput[Field] | null };
export const settingsApi = {
  get: () => fetchClient<ModelSettings>('/settings/models', { reportError: false, cache: 'no-store' }),
  detail: (id: string) => fetchClient<ModelConfig>('/settings/models/' + encodeURIComponent(id), { reportError: false, cache: 'no-store' }),
  create: (input: ModelInput) => postJson<{ id: string }>('/settings/models/create', input),
  update: (input: ModelUpdate) => postJson<void>('/settings/models/update', input),
  delete: (id: string) => postJson<void>('/settings/models/delete', { id }),
  saveRoute: (route: Route, model_id: string | null) => postJson<void>('/settings/routes/save', { route, model_id }),
};

export interface AgentSettings {
  auto_rename_enabled: boolean;
  auto_suggest_enabled: boolean;
  show_tool_failures: boolean;
  require_resource_approval: boolean;
  require_high_risk_approval: boolean;
}
export const agentSettingsApi = {
  get: () => fetchClient<AgentSettings>('/settings/behavior', { reportError: false, cache: 'no-store' }),
  save: (edit: { [Field in keyof AgentSettings]?: boolean | null }) => postJson<void>('/settings/behavior/save', edit),
};
