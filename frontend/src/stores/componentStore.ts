import { create } from 'zustand';
import { assetsApi, type AssetInfo } from '../api/assets';

interface ComponentState {
  components: AssetInfo[];
  loading: boolean;
  loaded: boolean;
  error: string;
  version: number;
  load: (force?: boolean) => Promise<AssetInfo[]>;
}

let loadPromise: Promise<AssetInfo[]> | null = null;

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
    loadPromise = assetsApi.list('component', '')
      .then((response) => {
        const components = response.items.filter(component => component.enabled);
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
