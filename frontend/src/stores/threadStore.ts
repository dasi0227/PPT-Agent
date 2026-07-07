import { create } from 'zustand';
import { Thread } from '../api/types';
import { threadsApi } from '../api/threads';
import { useRunStore } from './runStore';
import { isDraftId, newDraftId } from '../lib/draft';

interface ThreadState {
  // 存在态：来自后端（已 flush 落库）
  threadsByProjectId: Record<string, Thread[]>;
  // 草稿态：仅前端（软创建，未落库）
  draftThreadsByProjectId: Record<string, Thread[]>;
  // 打开态：每个 project 当前展开的标签顺序（含草稿）
  openThreadIdsByProjectId: Record<string, string[]>;
  // 每个 project 当前聚焦的 thread
  activeThreadIdByProjectId: Record<string, string | null>;

  loadThreads: (projectId: string) => Promise<void>;
  openThread: (projectId: string, threadId: string) => void;
  closeThread: (projectId: string, threadId: string) => void;
  createDraftThread: (projectId: string, title?: string) => string;
  flushThread: (projectId: string, tmpId: string) => Promise<string>;
  deleteThread: (projectId: string, threadId: string) => Promise<void>;
  setActiveThread: (projectId: string, threadId: string) => void;
  ensureActiveThread: (projectId: string) => Promise<string>;
  getActiveThreadId: (projectId: string) => string | null;
  displayThreads: (projectId: string) => Thread[];
  rekeyProject: (oldProjectId: string, newProjectId: string) => void;
  dropProject: (projectId: string) => void;
  nextUntitledName: (projectId: string) => string;
}

function withOpen(ids: string[], threadId: string): string[] {
  return ids.includes(threadId) ? ids : [...ids, threadId];
}

// flush 去重：同一草稿的并发 flush 只发一次请求（module 级，不入 state 避免重渲染）。
const pendingThreadFlush = new Map<string, Promise<string>>();

export const useThreadStore = create<ThreadState>((set, get) => ({
  threadsByProjectId: {},
  draftThreadsByProjectId: {},
  openThreadIdsByProjectId: {},
  activeThreadIdByProjectId: {},

  // 展示态 = 后端态 + 草稿（供 ThreadTabs 渲染）。
  displayThreads: (projectId) => [
    ...(get().threadsByProjectId[projectId] || []),
    ...(get().draftThreadsByProjectId[projectId] || []),
  ],

  loadThreads: async (projectId) => {
    // 草稿 project 不落库，跳过后端拉取（严禁把 draft_* 拼进 REST 路径）。
    if (isDraftId(projectId)) return;
    try {
      const threads = await threadsApi.list(projectId);
      set((state) => {
        const nextThreads = { ...state.threadsByProjectId, [projectId]: threads };
        // 若尚无打开态/聚焦态，默认打开最近活跃 thread（列表首个）。
        // 保留已有草稿 thread 的打开态。
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
    const isDraft = isDraftId(threadId);
    set((state) => {
      const open = state.openThreadIdsByProjectId[projectId] || [];
      const idx = open.indexOf(threadId);
      const nextOpen = open.filter((id) => id !== threadId);
      let active = state.activeThreadIdByProjectId[projectId] ?? null;
      if (active === threadId) {
        // 聚焦到相邻标签（优先右侧，其次左侧，否则 null）
        active = nextOpen[idx] ?? nextOpen[idx - 1] ?? null;
      }
      const patch: Partial<ThreadState> = {
        openThreadIdsByProjectId: { ...state.openThreadIdsByProjectId, [projectId]: nextOpen },
        activeThreadIdByProjectId: { ...state.activeThreadIdByProjectId, [projectId]: active },
      };
      // 关草稿 = 纯前端丢弃（删草稿项，零残留）；关真实 thread = 仅移出打开态。
      if (isDraft) {
        patch.draftThreadsByProjectId = {
          ...state.draftThreadsByProjectId,
          [projectId]: (state.draftThreadsByProjectId[projectId] || []).filter((t) => t.id !== threadId),
        };
      }
      return patch;
    });
    if (isDraft) useRunStore.getState().dropSessions([threadId]);
  },

  // 软创建：只建前端草稿项 + 加入打开态 + 设为 active，不发请求。
  createDraftThread: (projectId, title) => {
    const now = Math.floor(Date.now() / 1000);
    const draft: Thread = {
      id: newDraftId(),
      project_id: projectId,
      title: title || '新对话',
      created_at: now,
      updated_at: now,
      draft: true,
    };
    set((state) => ({
      draftThreadsByProjectId: {
        ...state.draftThreadsByProjectId,
        [projectId]: [...(state.draftThreadsByProjectId[projectId] || []), draft],
      },
      openThreadIdsByProjectId: {
        ...state.openThreadIdsByProjectId,
        [projectId]: withOpen(state.openThreadIdsByProjectId[projectId] || [], draft.id),
      },
      activeThreadIdByProjectId: { ...state.activeThreadIdByProjectId, [projectId]: draft.id },
    }));
    return draft.id;
  },

  // flush：POST 拿真实 thread，把临时 id 全量改绑为真实 id（session/open/active/展示态）。
  // 幂等：非草稿直接返回；并发重复 flush 复用同一 pending Promise。失败清理 pending 供重试。
  flushThread: async (projectId, tmpId) => {
    if (!isDraftId(tmpId)) return tmpId;

    const pending = pendingThreadFlush.get(tmpId);
    if (pending) return pending;

    const p = (async () => {
      const title = get().draftThreadsByProjectId[projectId]?.find((t) => t.id === tmpId)?.title;
      const thread = await threadsApi.create(projectId, title);
      // 改绑 MUST 在建立 SSE / createRun 之前完成。先迁移 run 分片，再改状态。
      useRunStore.getState().rekeySession(tmpId, thread.id);
      set((state) => {
        const drafts = (state.draftThreadsByProjectId[projectId] || []).filter((t) => t.id !== tmpId);
        const open = (state.openThreadIdsByProjectId[projectId] || []).map((id) => (id === tmpId ? thread.id : id));
        const active = state.activeThreadIdByProjectId[projectId] === tmpId
          ? thread.id
          : state.activeThreadIdByProjectId[projectId] ?? null;
        return {
          threadsByProjectId: {
            ...state.threadsByProjectId,
            [projectId]: [...(state.threadsByProjectId[projectId] || []), thread],
          },
          draftThreadsByProjectId: { ...state.draftThreadsByProjectId, [projectId]: drafts },
          openThreadIdsByProjectId: { ...state.openThreadIdsByProjectId, [projectId]: open },
          activeThreadIdByProjectId: { ...state.activeThreadIdByProjectId, [projectId]: active },
        };
      });
      return thread.id;
    })();

    pendingThreadFlush.set(tmpId, p);
    try {
      return await p;
    } finally {
      pendingThreadFlush.delete(tmpId);
    }
  },

  deleteThread: async (projectId, threadId) => {
    // 草稿无后端副本：等价于丢弃（不发 DELETE）。
    if (isDraftId(threadId)) {
      get().closeThread(projectId, threadId);
      return;
    }
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

  // 无副作用不落库：有真实 active 直接用；active 是草稿则 flush；无 active 时优先后端首个，否则建草稿再 flush。
  ensureActiveThread: async (projectId) => {
    const active = get().activeThreadIdByProjectId[projectId];
    if (active) {
      return isDraftId(active) ? get().flushThread(projectId, active) : active;
    }
    const threads = get().threadsByProjectId[projectId] || [];
    if (threads.length > 0) {
      get().setActiveThread(projectId, threads[0].id);
      return threads[0].id;
    }
    const tmpId = get().createDraftThread(projectId, get().nextUntitledName(projectId));
    return get().flushThread(projectId, tmpId);
  },

  getActiveThreadId: (projectId) => get().activeThreadIdByProjectId[projectId] ?? null,

  // project flush：把该 project 下所有 thread 状态从临时 projectId 改绑到真实 projectId。
  // thread id 不变（session 按 threadId 分片，无需 rekeySession）。
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
      const drafts = { ...state.draftThreadsByProjectId };
      if (oldProjectId in drafts) {
        drafts[newProjectId] = (drafts[oldProjectId] || []).map((t) => ({ ...t, project_id: newProjectId }));
        delete drafts[oldProjectId];
      }
      return {
        threadsByProjectId: move(state.threadsByProjectId),
        draftThreadsByProjectId: drafts,
        openThreadIdsByProjectId: move(state.openThreadIdsByProjectId),
        activeThreadIdByProjectId: move(state.activeThreadIdByProjectId),
      };
    });
  },

  // 生成下一个「未命名 N」标题：扫真实 + 草稿两侧当前存在的 title，取最大 N +1；
  // 序号不回收——删除历史 thread 后新建仍走最大 +1，避免复用他人已见过的标题。
  nextUntitledName: (projectId) => {
    const all = [
      ...(get().threadsByProjectId[projectId] || []),
      ...(get().draftThreadsByProjectId[projectId] || []),
    ];
    const re = /^未命名 (\d+)$/;
    const maxN = all.reduce((m, t) => {
      const match = (t.title || '').match(re);
      return match ? Math.max(m, parseInt(match[1], 10)) : m;
    }, 0);
    return `未命名 ${maxN + 1}`;
  },

  // 删除/丢弃 project：关闭其下所有 thread 的连接并清理全部 project 键（零残留）。
  dropProject: (projectId) => {
    const ids = new Set<string>([
      ...(get().openThreadIdsByProjectId[projectId] || []),
      ...(get().threadsByProjectId[projectId] || []).map((t) => t.id),
      ...(get().draftThreadsByProjectId[projectId] || []).map((t) => t.id),
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
        draftThreadsByProjectId: drop(state.draftThreadsByProjectId),
        openThreadIdsByProjectId: drop(state.openThreadIdsByProjectId),
        activeThreadIdByProjectId: drop(state.activeThreadIdByProjectId),
      };
    });
  },
}));
