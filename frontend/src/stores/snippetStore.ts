import { create } from 'zustand';
import { snippetsApi } from '../api/snippets';
import type { Snippet, SnippetWriteRequest } from '../api/types';

interface SnippetState {
  snippets: Snippet[];
  loading: boolean;
  loaded: boolean;
  error: string;
  version: number;
  load: (force?: boolean) => Promise<Snippet[]>;
  create: (request: SnippetWriteRequest) => Promise<Snippet>;
  update: (id: string, request: Pick<SnippetWriteRequest, 'name' | 'description' | 'tags'>) => Promise<Snippet>;
  setDisabled: (id: string, disabled: boolean) => Promise<Snippet>;
  delete: (id: string) => Promise<void>;
}

let loadPromise: Promise<Snippet[]> | null = null;

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
    loadPromise = snippetsApi.list()
      .then((response) => {
        set((state) => ({
          snippets: response.snippets,
          loading: false,
          loaded: true,
          error: '',
          version: state.version + 1,
        }));
        return response.snippets;
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
  create: async (request) => {
    const snippet = await snippetsApi.create(request);
    set((state) => ({
      snippets: [snippet, ...state.snippets],
      loaded: true,
      error: '',
      version: state.version + 1,
    }));
    return snippet;
  },
  update: async (id, request) => {
    const snippet = await snippetsApi.update(id, request);
    set((state) => ({
      snippets: [snippet, ...state.snippets.filter((value) => value.id !== id)],
      error: '',
      version: state.version + 1,
    }));
    return snippet;
  },
  setDisabled: async (id, disabled) => {
    const snippet = await snippetsApi.setDisabled(id, disabled);
    set((state) => ({
      snippets: state.snippets.map((value) => value.id === id ? snippet : value),
      error: '',
      version: state.version + 1,
    }));
    return snippet;
  },
  delete: async (id) => {
    await snippetsApi.delete(id);
    set((state) => ({
      snippets: state.snippets.filter((snippet) => snippet.id !== id),
      version: state.version + 1,
    }));
  },
}));
