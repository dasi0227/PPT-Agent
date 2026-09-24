import { create } from 'zustand';
import { fetchClient } from '../api/client';
import { defaultBindings, type ShortcutSettings } from '../lib/shortcuts';

const endpoint = '/settings/shortcuts';
let requestVersion = 0;
let loading: Promise<void> | undefined;
export const useShortcutStore = create<ShortcutSettings & {
  ready: boolean;
  load: () => Promise<void>;
  save: (edit: ShortcutSettings) => Promise<ShortcutSettings>;
}>((set, get) => ({
  revision: 0, bindings: defaultBindings, ready: false,
  load: () => {
    if (loading) return loading;
    const version = ++requestVersion;
    loading = fetchClient<ShortcutSettings>(endpoint, { reportError: false, cache: 'no-store' })
      .then(value => { if (version === requestVersion && (!get().ready || value.revision >= get().revision)) set({ ...value, ready: true }); })
      .finally(() => { loading = undefined; });
    return loading;
  },
  save: async edit => {
    ++requestVersion;
    const value = await fetchClient<ShortcutSettings>(endpoint, { method: 'PUT', body: JSON.stringify(edit), reportError: false });
    set({ ...value, ready: true });
    // Other windows refetch the database; the browser signal carries no settings data.
    try { localStorage.setItem('ppt-shortcuts-updated', `${value.revision}:${Date.now()}`); } catch { /* Database save already succeeded; focus refresh still synchronizes other windows. */ }
    return value;
  },
}));
