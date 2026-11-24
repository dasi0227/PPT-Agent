import { create } from 'zustand';
import { filesApi, type FileSettings } from '../api/files';

let generation = 0;
let loading: Promise<void> | undefined;
export const useFileSettingsStore = create<{
  value: FileSettings | null;
  load: () => Promise<void>;
  save: (edit: FileSettings) => Promise<void>;
}>((set) => ({
  value: null,
  load: () => {
    if (loading) return loading;
    const version = generation;
    loading = filesApi.get().then(value => {
      if (version === generation) set({ value });
    }).finally(() => { loading = undefined; });
    return loading;
  },
  save: async edit => {
    const value = await filesApi.save(edit);
    ++generation;
    set({ value });
    try { localStorage.setItem('ppt-file-settings-updated', String(Date.now())); } catch { /* Focus refresh also synchronizes windows. */ }
  },
}));
