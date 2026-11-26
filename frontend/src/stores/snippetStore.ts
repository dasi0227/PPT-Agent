import { create } from 'zustand';
import { assetsApi, type AssetDetail } from '../api/assets';

interface SnippetState {
  snippets: AssetDetail[];
  loading: boolean;
  loaded: boolean;
  error: string;
  version: number;
  load: (force?: boolean) => Promise<AssetDetail[]>;
}

let loadPromise: Promise<AssetDetail[]> | null = null;

export const useSnippetStore = create<SnippetState>((set, get) => ({
  snippets: [],
  loading: false,
  loaded: false,
  error: '',
  version: 0,
  load: async (force = false) => {
    if (!force && get().loaded) return get().snippets;
    if (loadPromise) return loadPromise;
    set({ loading: true, error: '' });
    loadPromise = assetsApi.list('snippet', '')
      .then(({ items }) => {
        const previous = new Map(get().snippets.map(snippet => [snippet.id, snippet]));
        return Promise.all(items.filter(item => item.enabled).map(item => {
          const cached = previous.get(item.id);
          return cached?.updated_at === item.updated_at
            ? { ...item, content: cached.content }
            : assetsApi.detail(item.id);
        }));
      })
      .then(snippets => {
        set((state) => ({
          snippets,
          loading: false,
          loaded: true,
          error: '',
          version: state.version + 1,
        }));
        return snippets;
      })
      .catch((cause: unknown) => {
        const message = cause instanceof Error ? cause.message : '短语加载失败';
        set({ loading: false, error: message });
        throw cause;
      })
      .finally(() => {
        loadPromise = null;
      });
    return loadPromise;
  },
}));
