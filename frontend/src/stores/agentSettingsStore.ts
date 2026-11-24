import { create } from 'zustand';
import { agentSettingsApi, type AgentSettings } from '../api/settings';

let generation = 0;

export const useAgentSettingsStore = create<{
  value: AgentSettings | null;
  load: () => Promise<void>;
  save: (edit: Partial<AgentSettings>) => Promise<void>;
}>((set) => ({
  value: null,
  load: async () => {
    const started = ++generation;
    const value = await agentSettingsApi.get();
    if (started === generation) set({ value });
  },
  save: async edit => {
    await agentSettingsApi.save(edit);
    ++generation;
    set(state => ({ value: state.value && { ...state.value, ...edit } }));
    try { localStorage.setItem('ppt-agent-settings-updated', String(Date.now())); } catch { /* Focus also refreshes. */ }
  },
}));
