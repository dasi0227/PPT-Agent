import { create } from 'zustand';
import { threadsApi } from '../api/threads';
import type { ContextWindowSnapshot } from '../api/types';
import { showGlobalError } from './toastStore';

interface ContextWindowSession {
  snapshot: ContextWindowSnapshot | null;
  loading: boolean;
  compacting: boolean;
}

interface ContextWindowState {
  sessions: Record<string, ContextWindowSession>;
  load: (threadId: string, modelProfileName: string) => Promise<void>;
  compact: (threadId: string, modelProfileName: string) => Promise<boolean>;
  update: (threadId: string, snapshot: ContextWindowSnapshot) => void;
  drop: (threadId: string) => void;
}

const emptySession = (): ContextWindowSession => ({
  snapshot: null,
  loading: false,
  compacting: false,
});

export const useContextWindowStore = create<ContextWindowState>((set) => ({
  sessions: {},
  load: async (threadId, modelProfileName) => {
    set((state) => ({
      sessions: {
        ...state.sessions,
        [threadId]: { ...(state.sessions[threadId] ?? emptySession()), loading: true },
      },
    }));
    try {
      const snapshot = await threadsApi.contextWindow(threadId, modelProfileName);
      set((state) => ({
        sessions: {
          ...state.sessions,
          [threadId]: { snapshot, loading: false, compacting: false },
        },
      }));
    } catch {
      set((state) => ({
        sessions: {
          ...state.sessions,
          [threadId]: { ...(state.sessions[threadId] ?? emptySession()), loading: false },
        },
      }));
    }
  },
  compact: async (threadId, modelProfileName) => {
    set((state) => ({
      sessions: {
        ...state.sessions,
        [threadId]: { ...(state.sessions[threadId] ?? emptySession()), compacting: true },
      },
    }));
    try {
      const result = await threadsApi.compact(threadId, modelProfileName);
      set((state) => ({
        sessions: {
          ...state.sessions,
          [threadId]: { snapshot: result.snapshot, loading: false, compacting: false },
        },
      }));
      return true;
    } catch (error) {
      showGlobalError(error instanceof Error ? error.message : '上下文压缩失败，请重试');
      set((state) => ({
        sessions: {
          ...state.sessions,
          [threadId]: { ...(state.sessions[threadId] ?? emptySession()), compacting: false },
        },
      }));
      return false;
    }
  },
  update: (threadId, snapshot) => set((state) => ({
    sessions: {
      ...state.sessions,
      [threadId]: { snapshot, loading: false, compacting: snapshot.status === 'compacting' },
    },
  })),
  drop: (threadId) => set((state) => {
    const sessions = { ...state.sessions };
    delete sessions[threadId];
    return { sessions };
  }),
}));
