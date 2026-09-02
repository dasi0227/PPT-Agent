import { create } from 'zustand';
import { promptsApi } from '../api/prompts';
import type { Prompt, PromptWriteRequest } from '../api/types';

export const PROMPT_RECENT_STORAGE_KEY = 'dasi.prompt-recent.v1';
const MAX_RECENT_PROMPTS = 20;

function readRecentIds(): string[] {
  if (typeof localStorage === 'undefined') return [];
  try {
    const value = JSON.parse(localStorage.getItem(PROMPT_RECENT_STORAGE_KEY) ?? '[]');
    return Array.isArray(value) ? value.filter((id): id is string => typeof id === 'string').slice(0, MAX_RECENT_PROMPTS) : [];
  } catch {
    return [];
  }
}

function writeRecentIds(ids: string[]) {
  if (typeof localStorage !== 'undefined') {
    localStorage.setItem(PROMPT_RECENT_STORAGE_KEY, JSON.stringify(ids));
  }
}

function reconcileRecent(ids: string[], prompts: Prompt[]): string[] {
  const valid = new Set(prompts.map((prompt) => prompt.id));
  return ids.filter((id, index) => valid.has(id) && ids.indexOf(id) === index).slice(0, MAX_RECENT_PROMPTS);
}

interface PromptState {
  prompts: Prompt[];
  recentIds: string[];
  loading: boolean;
  loaded: boolean;
  error: string;
  version: number;
  load: (force?: boolean) => Promise<Prompt[]>;
  create: (request: PromptWriteRequest) => Promise<Prompt>;
  update: (id: string, request: PromptWriteRequest) => Promise<Prompt>;
  setDisabled: (id: string, disabled: boolean) => Promise<Prompt>;
  delete: (id: string) => Promise<void>;
  recordRecent: (id: string) => void;
}

let loadPromise: Promise<Prompt[]> | null = null;

export const usePromptStore = create<PromptState>((set, get) => ({
  prompts: [],
  recentIds: readRecentIds(),
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
        const recentIds = reconcileRecent(get().recentIds, response.prompts);
        writeRecentIds(recentIds);
        set((state) => ({
          prompts: response.prompts,
          recentIds,
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
    const recentIds = get().recentIds.filter((value) => value !== id);
    writeRecentIds(recentIds);
    set((state) => ({
      prompts: state.prompts.filter((prompt) => prompt.id !== id),
      recentIds,
      version: state.version + 1,
    }));
  },
  recordRecent: (id) => {
    const recentIds = [id, ...get().recentIds.filter((value) => value !== id)].slice(0, MAX_RECENT_PROMPTS);
    writeRecentIds(recentIds);
    set({ recentIds });
  },
}));

export const promptStoreInternals = { readRecentIds, reconcileRecent };
