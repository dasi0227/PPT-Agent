import { create } from 'zustand';
import { repositoriesApi } from '../api/repositories';
import type { ComponentReference } from '../api/types';

interface ComponentState {
  components: ComponentReference[];
  loading: boolean;
  loaded: boolean;
  error: string;
  version: number;
  load: (force?: boolean) => Promise<ComponentReference[]>;
}

let loadPromise: Promise<ComponentReference[]> | null = null;

export const useComponentStore = create<ComponentState>((set, get) => ({
  components: [],
  loading: false,
  loaded: false,
  error: '',
  version: 0,
  load: async (force = false) => {
    if (!force && get().loaded) return get().components;
    if (loadPromise) return loadPromise;
    set({ loading: true, error: '' });
    loadPromise = repositoriesApi.listComponents()
      .then((response) => {
        const components = response.components.filter((component) => !component.disabled);
        set((state) => ({
          components,
          loading: false,
          loaded: true,
          error: '',
          version: state.version + 1,
        }));
        return components;
      })
      .catch((cause: unknown) => {
        const message = cause instanceof Error ? cause.message : '组件加载失败';
        set({ components: [], loading: false, error: message });
        throw cause;
      })
      .finally(() => {
        loadPromise = null;
      });
    return loadPromise;
  },
}));
