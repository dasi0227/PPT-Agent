import { compareRevision } from '../api/session';
import { generateNameCommand } from './textCommandStore';
import { create } from 'zustand';
import { Thread, ThreadNamingAction } from '../api/types';
import { threadsApi } from '../api/threads';
import { useRunStore } from './runStore';
import { newClientIdentity } from '../lib/clientIdentity';
import { isProjectDeleted, isThreadDeleted, markThreadDeleted, restoreThread } from './deletedTargets';
import { useContextWindowStore } from './contextWindowStore';
import { dropThreadCommands } from './commandRuntime';
import { rememberDeletion, recoverDeletionNotices } from './deletionStore';

interface RenamePanelTarget { projectId: string; threadId: string }

type ListedThread = Pick<Thread, 'id' | 'title' | 'updated_at'> & Partial<Thread>;
const threadListVersions = new Map<string, number>();

interface ThreadState {
  threadsByProjectId: Record<string, ListedThread[]>;
  openThreadIdsByProjectId: Record<string, string[]>;
  activeThreadIdByProjectId: Record<string, string | null>;
  errorByProjectId: Record<string, string | null>;
	renamePanelTarget: RenamePanelTarget | null;
	suggestPanelTarget: RenamePanelTarget | null;
	pendingNamingOperationByThreadId: Record<string, string>;

  loadThreads: (projectId: string) => Promise<void>;
  createThread: (projectId: string, title?: string) => Promise<string>;
  deleteThread: (projectId: string, threadId: string) => Promise<void>;
	openRenamePanel: (projectId: string, threadId: string) => void;
	closeRenamePanel: () => void;
	openSuggestPanel: (projectId: string, threadId: string) => void;
	closeSuggestPanel: () => void;
	setAutoSuggest: (projectId: string, threadId: string, enabled: boolean) => Promise<void>;
	performNamingAction: (projectId: string, threadId: string, action: ThreadNamingAction, title?: string) => Promise<void>;
  setActiveThread: (projectId: string, threadId: string) => void;
  ensureActiveThread: (projectId: string) => Promise<string>;
  getActiveThreadId: (projectId: string) => string | null;
  displayThreads: (projectId: string) => ListedThread[];
  dropProject: (projectId: string) => string[];
}

function withOpen(ids: string[], threadId: string): string[] {
  return ids.includes(threadId) ? ids : [...ids, threadId];
}

export const useThreadStore = create<ThreadState>((set, get) => ({
  threadsByProjectId: {},
  openThreadIdsByProjectId: {},
  activeThreadIdByProjectId: {},
  errorByProjectId: {},
	renamePanelTarget: null,
	suggestPanelTarget: null,
	pendingNamingOperationByThreadId: {},

  displayThreads: (projectId) => get().threadsByProjectId[projectId] || [],

  loadThreads: async (projectId) => {
    if (isProjectDeleted(projectId)) return;
    const version = (threadListVersions.get(projectId) ?? 0) + 1;
    threadListVersions.set(projectId, version);
    try {
      const threads = (await threadsApi.list(projectId)).filter(thread => !isThreadDeleted(thread.id));
      if (isProjectDeleted(projectId) || threadListVersions.get(projectId) !== version) return;
      set((state) => {
        const previous = new Map((state.threadsByProjectId[projectId] ?? []).map(thread => [thread.id, thread]));
        const nextThreads = { ...state.threadsByProjectId, [projectId]: threads.map(thread => ({ ...previous.get(thread.id), ...thread })) };
        const valid = new Set(threads.map((thread) => thread.id));
        const alreadyOpen = state.openThreadIdsByProjectId[projectId]?.filter((id) => valid.has(id));
        const nextOpen = { ...state.openThreadIdsByProjectId };
        const nextActive = { ...state.activeThreadIdByProjectId };
        nextOpen[projectId] = alreadyOpen ?? [];
        if (!nextActive[projectId] || !valid.has(nextActive[projectId]!)) nextActive[projectId] = threads[0]?.id ?? null;
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
      if (isProjectDeleted(projectId) || threadListVersions.get(projectId) !== version) return;
      set((state) => ({
        errorByProjectId: {
          ...state.errorByProjectId,
          [projectId]: err instanceof Error ? err.message : '会话加载失败，请重试',
        },
      }));
    }
  },

  createThread: async (projectId, title) => {
    const thread = await threadsApi.create(projectId, title);
    if (isProjectDeleted(projectId)) { markThreadDeleted(thread.id); return thread.id; }
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

	openRenamePanel: (projectId, threadId) => set({ renamePanelTarget: { projectId, threadId }, suggestPanelTarget: null }),
	closeRenamePanel: () => set({ renamePanelTarget: null }),
	openSuggestPanel: (projectId, threadId) => set({ suggestPanelTarget: { projectId, threadId }, renamePanelTarget: null }),
	closeSuggestPanel: () => set({ suggestPanelTarget: null }),
	setAutoSuggest: async (projectId, threadId, enabled) => {
		const updated = await threadsApi.setAutoSuggest(threadId, enabled);
		set(state => ({ threadsByProjectId: { ...state.threadsByProjectId,
			[projectId]: (state.threadsByProjectId[projectId] ?? []).map(thread =>
				thread.id === threadId && (!thread.naming_revision || compareRevision(updated.naming_revision, thread.naming_revision) >= 0) ? updated : thread),
		} }));
	},

  performNamingAction: async (projectId, threadId, action, title) => {
    const apply = (updated: Thread) => set(state => ({
      threadsByProjectId: {
        ...state.threadsByProjectId,
        [projectId]: (state.threadsByProjectId[projectId] ?? []).map(thread =>
          thread.id === threadId && (!thread.naming_revision || compareRevision(updated.naming_revision, thread.naming_revision) >= 0) ? updated : thread),
      },
    }));
    if (action === 'generate') {
      if (get().pendingNamingOperationByThreadId[threadId]) return;
      const marker = newClientIdentity('rename');
      set(state => ({ pendingNamingOperationByThreadId: { ...state.pendingNamingOperationByThreadId, [threadId]: marker } }));
      try {
        await generateNameCommand(threadId, apply);
      } finally {
        set(state => {
          const pending = { ...state.pendingNamingOperationByThreadId };
          if (pending[threadId] === marker) delete pending[threadId];
          return { pendingNamingOperationByThreadId: pending };
        });
      }
      return;
    }
    const operationId = newClientIdentity('rename');
    apply(await threadsApi.naming(threadId, operationId, action, title));

  },

  deleteThread: async (projectId, threadId) => {
    if (isThreadDeleted(threadId)) return;
    markThreadDeleted(threadId);
    set((state) => {
      const open = state.openThreadIdsByProjectId[projectId] || [];
      const idx = open.indexOf(threadId);
      const nextOpen = open.filter((id) => id !== threadId);
      let active = state.activeThreadIdByProjectId[projectId] ?? null;
      if (active === threadId) {
        active = nextOpen[idx] ?? nextOpen[idx - 1] ?? null;
      }
	  const pending = { ...state.pendingNamingOperationByThreadId };
	  delete pending[threadId];
      return {
        threadsByProjectId: {
          ...state.threadsByProjectId,
          [projectId]: (state.threadsByProjectId[projectId] || []).filter((t) => t.id !== threadId),
        },
        openThreadIdsByProjectId: { ...state.openThreadIdsByProjectId, [projectId]: nextOpen },
        activeThreadIdByProjectId: { ...state.activeThreadIdByProjectId, [projectId]: active },
		pendingNamingOperationByThreadId: pending,
		renamePanelTarget: state.renamePanelTarget?.threadId === threadId ? null : state.renamePanelTarget,
		suggestPanelTarget: state.suggestPanelTarget?.threadId === threadId ? null : state.suggestPanelTarget,
      };
    });
    useRunStore.getState().dropSessions([threadId]);
    useContextWindowStore.getState().drop(threadId);
    dropThreadCommands([threadId]);
    const startedAt = Date.now();
    try { rememberDeletion(await threadsApi.delete(threadId), startedAt); }
    catch (error) {
      try {
        const threads = await threadsApi.list(projectId);
        if (threads.some(thread => thread.id === threadId)) {
          restoreThread(threadId);
          await get().loadThreads(projectId);
        }
      } catch { /* Keep the fence when acceptance could not be reconciled. */ }
      void recoverDeletionNotices().catch(() => {});
      throw error;
    }
  },

  setActiveThread: (projectId, threadId) => {
    if (isProjectDeleted(projectId) || isThreadDeleted(threadId)) return;
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

  dropProject: (projectId) => {
    threadListVersions.delete(projectId);
    const ids = new Set<string>([
      ...(get().openThreadIdsByProjectId[projectId] || []),
      ...(get().threadsByProjectId[projectId] || []).map((t) => t.id),
      ...Object.entries(useRunStore.getState().sessions).filter(([, session]) => session.projectId === projectId).map(([id]) => id),
    ]);
    ids.forEach(markThreadDeleted);
    ids.forEach(id => useContextWindowStore.getState().drop(id));
    dropThreadCommands([...ids]);
    useRunStore.getState().dropSessions([...ids]);
    set((state) => {
      const drop = <T,>(m: Record<string, T>): Record<string, T> => {
        if (!(projectId in m)) return m;
        const next = { ...m };
        delete next[projectId];
        return next;
      };
	  const pending = { ...state.pendingNamingOperationByThreadId };
	  for (const threadId of ids) delete pending[threadId];
      return {
        threadsByProjectId: drop(state.threadsByProjectId),
        openThreadIdsByProjectId: drop(state.openThreadIdsByProjectId),
        activeThreadIdByProjectId: drop(state.activeThreadIdByProjectId),
		pendingNamingOperationByThreadId: pending,
		renamePanelTarget: state.renamePanelTarget?.projectId === projectId ? null : state.renamePanelTarget,
		suggestPanelTarget: state.suggestPanelTarget?.projectId === projectId ? null : state.suggestPanelTarget,
      };
    });
    return [...ids];
  },
}));
