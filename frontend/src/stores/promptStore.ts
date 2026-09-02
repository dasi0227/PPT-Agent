import { create } from 'zustand';
import { promptsApi } from '../api/prompts';
import type { Prompt, PromptWriteRequest } from '../api/types';

interface PromptState {
  prompts: Prompt[];
  loading: boolean;
  loaded: boolean;
  error: string;
  version: number;
  load: (force?: boolean) => Promise<Prompt[]>;
  create: (request: PromptWriteRequest) => Promise<Prompt>;
  update: (id: string, request: PromptWriteRequest) => Promise<Prompt>;
  setDisabled: (id: string, disabled: boolean) => Promise<Prompt>;
  delete: (id: string) => Promise<void>;
}

let loadPromise: Promise<Prompt[]> | null = null;

export const usePromptStore = create<PromptState>((set, get) => ({
  prompts: [],
  loading: false,
  loaded: false,
  error: '',
  version: 0,
  load: async (force = false) => {
    if (!force && get().loaded) return get().prompts;
    if (loadPromise) return loadPromise;
    set({ loading: true, error: '' });
    loadPromise = promptsApi.list()
      .then((response) => {
        set((state) => ({
          prompts: response.prompts,
          loading: false,
          loaded: true,
          error: '',
          version: state.version + 1,
        }));
        return response.prompts;
      })
      .catch((cause: unknown) => {
        const message = cause instanceof Error ? cause.message : '提示词加载失败';
        set({ loading: false, error: message });
        throw cause;
      })
      .finally(() => {
        loadPromise = null;
      });
    return loadPromise;
  },
  create: async (request) => {
    const prompt = await promptsApi.create(request);
    set((state) => ({
      prompts: [prompt, ...state.prompts],
      loaded: true,
      error: '',
      version: state.version + 1,
    }));
    return prompt;
  },
  update: async (id, request) => {
    const prompt = await promptsApi.update(id, request);
    set((state) => ({
      prompts: [prompt, ...state.prompts.filter((value) => value.id !== id)],
      error: '',
      version: state.version + 1,
    }));
    return prompt;
  },
  setDisabled: async (id, disabled) => {
    const prompt = await promptsApi.setDisabled(id, disabled);
    set((state) => ({
      prompts: state.prompts.map((value) => value.id === id ? prompt : value),
      error: '',
      version: state.version + 1,
    }));
    return prompt;
  },
  delete: async (id) => {
    await promptsApi.delete(id);
    set((state) => ({
      prompts: state.prompts.filter((prompt) => prompt.id !== id),
      version: state.version + 1,
    }));
  },
}));
