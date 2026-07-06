import { create } from 'zustand';
import { Thread } from '../api/types';
import { threadsApi } from '../api/threads';

interface ThreadState {
  // 存在态：来自后端
  threadsByProjectId: Record<string, Thread[]>;
  // 打开态：每个 project 当前展开的标签顺序
  openThreadIdsByProjectId: Record<string, string[]>;
  // 每个 project 当前聚焦的 thread
  activeThreadIdByProjectId: Record<string, string | null>;

  loadThreads: (projectId: string) => Promise<void>;
  openThread: (projectId: string, threadId: string) => void;
  closeThread: (projectId: string, threadId: string) => void;
  createThread: (projectId: string, title?: string) => Promise<string>;
  deleteThread: (projectId: string, threadId: string) => Promise<void>;
  setActiveThread: (projectId: string, threadId: string) => void;
  ensureActiveThread: (projectId: string) => Promise<string>;
  getActiveThreadId: (projectId: string) => string | null;
}

function withOpen(ids: string[], threadId: string): string[] {
  return ids.includes(threadId) ? ids : [...ids, threadId];
}

export const useThreadStore = create<ThreadState>((set, get) => ({
  threadsByProjectId: {},
  openThreadIdsByProjectId: {},
  activeThreadIdByProjectId: {},

  loadThreads: async (projectId) => {
    try {
      const threads = await threadsApi.list(projectId);
      set((state) => {
        const nextThreads = { ...state.threadsByProjectId, [projectId]: threads };
        // 若尚无打开态/聚焦态，默认打开最近活跃 thread（列表首个）
        const alreadyOpen = state.openThreadIdsByProjectId[projectId];
        const nextOpen = { ...state.openThreadIdsByProjectId };
        const nextActive = { ...state.activeThreadIdByProjectId };
        if ((!alreadyOpen || alreadyOpen.length === 0) && threads.length > 0) {
          nextOpen[projectId] = [threads[0].id];
          if (!nextActive[projectId]) nextActive[projectId] = threads[0].id;
        }
        return {
          threadsByProjectId: nextThreads,
          openThreadIdsByProjectId: nextOpen,
          activeThreadIdByProjectId: nextActive,
        };
      });
    } catch (err) {
      console.error('Failed to load threads', err);
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
        // 聚焦到相邻标签（优先右侧，其次左侧，否则 null）
        active = nextOpen[idx] ?? nextOpen[idx - 1] ?? null;
      }
      return {
        openThreadIdsByProjectId: { ...state.openThreadIdsByProjectId, [projectId]: nextOpen },
        activeThreadIdByProjectId: { ...state.activeThreadIdByProjectId, [projectId]: active },
      };
    });
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

  // 无活跃 thread 时兜底：优先已有 thread，否则自动建"主线程"。替代 v1 threads[0] 硬取。
  ensureActiveThread: async (projectId) => {
    const existing = get().activeThreadIdByProjectId[projectId];
    if (existing) return existing;

    const threads = get().threadsByProjectId[projectId] || [];
    if (threads.length > 0) {
      get().setActiveThread(projectId, threads[0].id);
      return threads[0].id;
    }
    return get().createThread(projectId, '主线程');
  },

  getActiveThreadId: (projectId) => get().activeThreadIdByProjectId[projectId] ?? null,
}));
