import { create } from 'zustand';
import { filesApi, type FileSettings, type FileSettingsView } from '../api/files';

let loading: Promise<void> | undefined;
export const useFileSettingsStore = create<{
  value: FileSettingsView | null;
  load: () => Promise<void>;
  save: (edit: FileSettings) => Promise<void>;
}>((set, get) => ({
  value: null,
  load: () => {
    if (loading) return loading;
    loading = filesApi.get().then(value => {
      if (value.revision >= (get().value?.revision ?? -1)) set({ value });
    }).finally(() => { loading = undefined; });
    return loading;
  },
  save: async edit => {
    const value = await filesApi.save(edit);
    if (value.revision >= (get().value?.revision ?? -1)) set({ value });
    try { localStorage.setItem('ppt-file-settings-updated', `${value.revision}:${Date.now()}`); } catch { /* Focus refresh also synchronizes windows. */ }
  },
}));
