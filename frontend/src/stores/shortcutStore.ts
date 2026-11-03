import { create } from 'zustand';
import { fetchClient } from '../api/client';
import { defaultBindings, type ShortcutSettings } from '../lib/shortcuts';
import { showGlobalError, useToastStore } from './toastStore';

const endpoint = '/settings/shortcuts';
let requestVersion = 0;
let loading: Promise<void> | undefined;
let pendingWrite: Promise<ShortcutSettings> | undefined;
let resetNotice: { toastId: string; revision: number; expiresAt: number } | undefined;

export function clearShortcutResetUndo() {
  if (resetNotice) useToastStore.getState().removeToast(resetNotice.toastId);
  resetNotice = undefined;
}

interface ShortcutState extends ShortcutSettings {
  ready: boolean;
  saving: boolean;
  load: () => Promise<void>;
  save: (edit: ShortcutSettings) => Promise<ShortcutSettings>;
  reset: () => Promise<ShortcutSettings>;
}

export const useShortcutStore = create<ShortcutState>((set, get) => {
  const write = (action: () => Promise<ShortcutSettings>) => {
    const task = (pendingWrite ?? Promise.resolve()).catch(() => {}).then(action);
    pendingWrite = task;
    set({ saving: true });
    return task.finally(() => {
      if (pendingWrite === task) {
        pendingWrite = undefined;
        set({ saving: false });
      }
    });
  };
  const persist = async (edit: ShortcutSettings) => {
    clearShortcutResetUndo();
    ++requestVersion;
    const value = await fetchClient<ShortcutSettings>(endpoint, { method: 'PUT', body: JSON.stringify(edit), reportError: false });
    set({ ...value, ready: true });
    // Other windows refetch the database; the browser signal carries no settings data.
    try { localStorage.setItem('ppt-shortcuts-updated', `${value.revision}:${Date.now()}`); } catch { /* Database save already succeeded; focus refresh still synchronizes other windows. */ }
    return value;
  };
  return {
    revision: 0, bindings: defaultBindings, ready: false, saving: false,
    load: () => {
      if (loading) return loading;
      const version = ++requestVersion;
      loading = fetchClient<ShortcutSettings>(endpoint, { reportError: false, cache: 'no-store' })
        .then(value => {
          if (version !== requestVersion || (get().ready && value.revision < get().revision)) return;
          if (resetNotice && value.revision !== resetNotice.revision) clearShortcutResetUndo();
          set({ ...value, ready: true });
        })
        .finally(() => { loading = undefined; });
      return loading;
    },
    save: edit => write(() => persist(edit)),
    reset: () => write(async () => {
      await get().load();
      const previousBindings = structuredClone(get().bindings);
      const value = await persist({ revision: get().revision, bindings: defaultBindings });
      const notice = { toastId: '', revision: value.revision, expiresAt: Date.now() + 5_000 };
      notice.toastId = useToastStore.getState().pushToast('快捷键已恢复默认', 'success', {
        label: '撤销',
        onClick: () => {
          if (resetNotice !== notice || Date.now() >= notice.expiresAt) return;
          clearShortcutResetUndo();
          void write(async () => {
            if (get().revision !== notice.revision) throw new Error('快捷键已更新，无法撤销此次恢复默认');
            return persist({ revision: notice.revision, bindings: previousBindings });
          }).catch(cause => showGlobalError(cause instanceof Error ? cause.message : '撤销失败，请重试'));
        },
      }, { expiresAt: notice.expiresAt });
      resetNotice = notice;
      return value;
    }),
  };
});
