import { create } from 'zustand';
import { agentSettingsApi, type AgentSettings } from '../api/settings';

let generation = 0;

export const useAgentSettingsStore = create<{
  value: AgentSettings | null;
  load: (reload?: boolean) => Promise<void>;
  save: (edit: AgentSettings) => Promise<void>;
}>((set) => ({
  value: null,
  load: async (reload = false) => {
    const started = ++generation;
    const value = await (reload ? agentSettingsApi.reload() : agentSettingsApi.get());
    if (started === generation) set({ value });
  },
  save: async edit => {
    const started = ++generation;
    try {
      const value = await agentSettingsApi.save(edit);
      const refreshRequired = started !== generation;
      generation += 1;
      set({ value });
      try { localStorage.setItem('ppt-agent-settings-updated', `${value.revision}:${Date.now()}`); } catch { /* Focus refresh also synchronizes windows. */ }
      if (refreshRequired) {
        const refreshed = ++generation;
        await agentSettingsApi.get().then(latest => {
          if (refreshed === generation) set({ value: latest });
        }).catch(() => {});
      }
    } catch (cause) { generation += 1; throw cause; }
  },
}));
