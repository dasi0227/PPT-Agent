import { create } from 'zustand';
import { Thread } from '../api/types';
import { threadsApi } from '../api/threads';
import { useRunStore } from './runStore';

interface ThreadState {
  threadsByProjectId: Record<string, Thread[]>;
  openThreadIdsByProjectId: Record<string, string[]>;
  activeThreadIdByProjectId: Record<string, string | null>;
  errorByProjectId: Record<string, string | null>;

  loadThreads: (projectId: string) => Promise<void>;
  openThread: (projectId: string, threadId: string) => void;
  closeThread: (projectId: string, threadId: string) => void;
  createThread: (projectId: string, title?: string) => Promise<string>;
  deleteThread: (projectId: string, threadId: string) => Promise<void>;
  renameThread: (projectId: string, threadId: string, title: string) => Promise<void>;
  setActiveThread: (projectId: string, threadId: string) => void;
  ensureActiveThread: (projectId: string) => Promise<string>;
  getActiveThreadId: (projectId: string) => string | null;
  displayThreads: (projectId: string) => Thread[];
  closeProjectThreads: (projectId: string) => void;
  rekeyProject: (oldProjectId: string, newProjectId: string) => void;
  dropProject: (projectId: string) => void;
}

function withOpen(ids: string[], threadId: string): string[] {
  return ids.includes(threadId) ? ids : [...ids, threadId];
}

export const useThreadStore = create<ThreadState>((set, get) => ({
  threadsByProjectId: {},
  openThreadIdsByProjectId: {},
  activeThreadIdByProjectId: {},
  errorByProjectId: {},

  displayThreads: (projectId) => get().threadsByProjectId[projectId] || [],

  loadThreads: async (projectId) => {
    try {
      const threads = await threadsApi.list(projectId);
      set((state) => {
        const nextThreads = { ...state.threadsByProjectId, [projectId]: threads };
        const alreadyOpen = state.openThreadIdsByProjectId[projectId];
        const nextOpen = { ...state.openThreadIdsByProjectId };
        const nextActive = { ...state.activeThreadIdByProjectId };
        if ((!alreadyOpen || alreadyOpen.length === 0) && threads.length > 0) {
          nextOpen[projectId] = threads.map((thread) => thread.id);
          if (!nextActive[projectId]) nextActive[projectId] = threads[0].id;
        }
        return {
          threadsByProjectId: nextThreads,
          openThreadIdsByProjectId: nextOpen,
          activeThreadIdByProjectId: nextActive,
          errorByProjectId: { ...state.errorByProjectId, [projectId]: null },
        };
      });
    } catch (err) {
      set((state) => ({
        errorByProjectId: {
          ...state.errorByProjectId,
          [projectId]: err instanceof Error ? err.message : '会话加载失败，请重试',
        },
      }));
    }
  },

  openThread: (projectId, threadId) => {
    set((state) => ({
      openThreadIdsByProjectId: {
        ...state.openThreadIdsByProjectId,
        [projectId]: withOpen(state.openThreadIdsByProjectId[projectId] || [], threadId),
      },
      activeThreadIdByProjectId: { ...state.activeThreadIdByProjectId, [projectId]: threadId },
    }));
  },

  closeThread: (projectId, threadId) => {
    set((state) => {
      const open = state.openThreadIdsByProjectId[projectId] || [];
      const idx = open.indexOf(threadId);
      const nextOpen = open.filter((id) => id !== threadId);
      let active = state.activeThreadIdByProjectId[projectId] ?? null;
      if (active === threadId) {
        active = nextOpen[idx] ?? nextOpen[idx - 1] ?? null;
      }
      return {
        openThreadIdsByProjectId: { ...state.openThreadIdsByProjectId, [projectId]: nextOpen },
        activeThreadIdByProjectId: { ...state.activeThreadIdByProjectId, [projectId]: active },
      };
    });
    useRunStore.getState().dropSessions([threadId]);
  },

  createThread: async (projectId, title) => {
    const thread = await threadsApi.create(projectId, title);
    set((state) => ({
      threadsByProjectId: {
        ...state.threadsByProjectId,
        [projectId]: [...(state.threadsByProjectId[projectId] || []), thread],
      },
      openThreadIdsByProjectId: {
        ...state.openThreadIdsByProjectId,
        [projectId]: withOpen(state.openThreadIdsByProjectId[projectId] || [], thread.id),
      },
      activeThreadIdByProjectId: { ...state.activeThreadIdByProjectId, [projectId]: thread.id },
    }));
    return thread.id;
  },

  renameThread: async (projectId, threadId, title) => {
    await threadsApi.patch(threadId, { title });
    set((state) => {
      const threads = state.threadsByProjectId[projectId] || [];
      return {
        threadsByProjectId: {
          ...state.threadsByProjectId,
          [projectId]: threads.map(t => t.id === threadId ? { ...t, title, updated_at: Math.floor(Date.now()/1000) } : t)
        }
      };
    });
  },

  deleteThread: async (projectId, threadId) => {
    await threadsApi.delete(threadId);
    set((state) => {
      const open = state.openThreadIdsByProjectId[projectId] || [];
      const idx = open.indexOf(threadId);
      const nextOpen = open.filter((id) => id !== threadId);
      let active = state.activeThreadIdByProjectId[projectId] ?? null;
      if (active === threadId) {
        active = nextOpen[idx] ?? nextOpen[idx - 1] ?? null;
      }
      return {
        threadsByProjectId: {
          ...state.threadsByProjectId,
          [projectId]: (state.threadsByProjectId[projectId] || []).filter((t) => t.id !== threadId),
        },
        openThreadIdsByProjectId: { ...state.openThreadIdsByProjectId, [projectId]: nextOpen },
        activeThreadIdByProjectId: { ...state.activeThreadIdByProjectId, [projectId]: active },
      };
    });
    useRunStore.getState().dropSessions([threadId]);
  },

  setActiveThread: (projectId, threadId) => {
    set((state) => ({
      openThreadIdsByProjectId: {
        ...state.openThreadIdsByProjectId,
        [projectId]: withOpen(state.openThreadIdsByProjectId[projectId] || [], threadId),
      },
      activeThreadIdByProjectId: { ...state.activeThreadIdByProjectId, [projectId]: threadId },
    }));
  },

  ensureActiveThread: async (projectId) => {
    const active = get().activeThreadIdByProjectId[projectId];
    if (active) {
      return active;
    }
    const threads = get().threadsByProjectId[projectId] || [];
    if (threads.length > 0) {
      get().setActiveThread(projectId, threads[0].id);
      return threads[0].id;
    }
    return await get().createThread(projectId, ''); // title empty will be defaulted by backend
  },

  getActiveThreadId: (projectId) => get().activeThreadIdByProjectId[projectId] ?? null,

  closeProjectThreads: (projectId) => {
    const ids = new Set<string>([
      ...(get().openThreadIdsByProjectId[projectId] || []),
      ...(get().threadsByProjectId[projectId] || []).map((t) => t.id),
    ]);
    useRunStore.getState().closeSessions([...ids]);
  },

  rekeyProject: (oldProjectId, newProjectId) => {
    if (oldProjectId === newProjectId) return;
    set((state) => {
      const move = <T,>(m: Record<string, T>): Record<string, T> => {
        if (!(oldProjectId in m)) return m;
        const next = { ...m };
        next[newProjectId] = next[oldProjectId];
        delete next[oldProjectId];
        return next;
      };
      return {
        threadsByProjectId: move(state.threadsByProjectId),
        openThreadIdsByProjectId: move(state.openThreadIdsByProjectId),
        activeThreadIdByProjectId: move(state.activeThreadIdByProjectId),
      };
    });
  },

  dropProject: (projectId) => {
    const ids = new Set<string>([
      ...(get().openThreadIdsByProjectId[projectId] || []),
      ...(get().threadsByProjectId[projectId] || []).map((t) => t.id),
    ]);
    useRunStore.getState().dropSessions([...ids]);
    set((state) => {
      const drop = <T,>(m: Record<string, T>): Record<string, T> => {
        if (!(projectId in m)) return m;
        const next = { ...m };
        delete next[projectId];
        return next;
      };
      return {
        threadsByProjectId: drop(state.threadsByProjectId),
        openThreadIdsByProjectId: drop(state.openThreadIdsByProjectId),
        activeThreadIdByProjectId: drop(state.activeThreadIdByProjectId),
      };
    });
  },
}));
