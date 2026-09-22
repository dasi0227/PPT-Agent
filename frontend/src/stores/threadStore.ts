import { generateNameCommand } from './textCommandStore';
import { upsertCommand } from './commandRuntime';
import { create } from 'zustand';
import { Thread, ThreadNamingAction } from '../api/types';
import { threadsApi } from '../api/threads';
import { useRunStore } from './runStore';
import { newClientIdentity } from '../lib/clientIdentity';
import { showGlobalError } from './toastStore';

interface RenamePanelTarget { projectId: string; threadId: string }

interface ThreadStreamState { epoch: string; sequence: number }
const streamStateByProject = new Map<string, ThreadStreamState>();
const eventSourceByProject = new Map<string, EventSource>();

interface ThreadState {
  threadsByProjectId: Record<string, Thread[]>;
  openThreadIdsByProjectId: Record<string, string[]>;
  activeThreadIdByProjectId: Record<string, string | null>;
  errorByProjectId: Record<string, string | null>;
	renamePanelTarget: RenamePanelTarget | null;
	pendingNamingOperationByThreadId: Record<string, string>;

  loadThreads: (projectId: string) => Promise<void>;
  openThread: (projectId: string, threadId: string) => void;
  closeThread: (projectId: string, threadId: string) => void;
  createThread: (projectId: string, title?: string) => Promise<string>;
  deleteThread: (projectId: string, threadId: string) => Promise<void>;
  renameThread: (projectId: string, threadId: string, title: string) => Promise<void>;
	openRenamePanel: (projectId: string, threadId: string) => void;
	closeRenamePanel: () => void;
	performNamingAction: (projectId: string, threadId: string, action: ThreadNamingAction, title?: string) => Promise<void>;
	applyThreadSnapshot: (projectId: string, epoch: string, sequence: number, threads: Thread[]) => void;
	applyThreadNamingUpdate: (projectId: string, epoch: string, sequence: number, thread: Thread) => void;
	applyThreadNamingResult: (projectId: string, epoch: string, sequence: number, data: Record<string, unknown>) => void;
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

function isThread(value: unknown): value is Thread {
	if (!value || typeof value !== 'object') return false;
	const thread = value as Partial<Thread>;
	return typeof thread.id === 'string' && typeof thread.project_id === 'string'
		&& typeof thread.title === 'string' && typeof thread.auto_rename_enabled === 'boolean'
		&& typeof thread.naming_revision === 'number';
}

function parseThreadEvent(projectId: string, name: string, raw: string) {
	let data: Record<string, unknown>;
	try {
		data = JSON.parse(raw) as Record<string, unknown>;
	} catch {
		return;
	}
	if (data.schema_version !== 1 || data.project_id !== projectId
		|| typeof data.stream_epoch !== 'string' || typeof data.sequence !== 'number') return;
	const epoch = data.stream_epoch;
	const sequence = data.sequence;
	if (name === 'threads.snapshot') {
		if (!Array.isArray(data.threads) || !data.threads.every(isThread)) return;
		useThreadStore.getState().applyThreadSnapshot(projectId, epoch, sequence, data.threads);
		return;
	}
	if (name === 'thread.naming.updated' && isThread(data.thread)) {
		useThreadStore.getState().applyThreadNamingUpdate(projectId, epoch, sequence, data.thread);
		return;
	}
	if (name === 'thread.naming.result') {
		useThreadStore.getState().applyThreadNamingResult(projectId, epoch, sequence, data);
	}
}

function ensureThreadEvents(projectId: string) {
	if (typeof EventSource === 'undefined') return;
	if (eventSourceByProject.has(projectId)) return;
	const source = new EventSource(`/api/v1/projects/${encodeURIComponent(projectId)}/thread-events`);
	for (const name of ['threads.snapshot', 'thread.naming.updated', 'thread.naming.result']) {
		source.addEventListener(name, (event) => parseThreadEvent(projectId, name, (event as MessageEvent<string>).data));
	}
	source.onerror = () => {
		// EventSource reconnects automatically; the server starts every connection with an authoritative snapshot.
	};
	eventSourceByProject.set(projectId, source);
}

function closeThreadEvents(projectId: string) {
	eventSourceByProject.get(projectId)?.close();
	eventSourceByProject.delete(projectId);
	streamStateByProject.delete(projectId);
}

export const useThreadStore = create<ThreadState>((set, get) => ({
  threadsByProjectId: {},
  openThreadIdsByProjectId: {},
  activeThreadIdByProjectId: {},
  errorByProjectId: {},
	renamePanelTarget: null,
	pendingNamingOperationByThreadId: {},

  displayThreads: (projectId) => get().threadsByProjectId[projectId] || [],

  loadThreads: async (projectId) => {
    try {
      const threads = await threadsApi.list(projectId);
      set((state) => {
        const nextThreads = { ...state.threadsByProjectId, [projectId]: threads };
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
		ensureThreadEvents(projectId);
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
    const previousTitle=get().threadsByProjectId[projectId]?.find(thread=>thread.id===threadId)?.title ?? '';
    const commandId = `rename:${newClientIdentity('rename')}`;
		const updated = await threadsApi.patch(threadId, { title }, commandId);
    set((state) => {
      const threads = state.threadsByProjectId[projectId] || [];
      return {
        threadsByProjectId: {
			...state.threadsByProjectId,
			[projectId]: threads.map((thread) => thread.id === threadId && updated.naming_revision >= thread.naming_revision ? updated : thread),
        }
      };
    });
    upsertCommand(threadId,{id:commandId,type:'command',kind:'rename',status:'completed',title:updated.title,content:`${previousTitle || '新会话'} → ${updated.title}`,method:'manual',timestamp:Date.now()});
  },

	openRenamePanel: (projectId, threadId) => set({ renamePanelTarget: { projectId, threadId } }),
	closeRenamePanel: () => set({ renamePanelTarget: null }),

  performNamingAction: async (projectId, threadId, action, title) => {
    const previousTitle = get().threadsByProjectId[projectId]?.find(thread => thread.id === threadId)?.title ?? '';
    const apply = (updated: Thread) => set(state => ({
      threadsByProjectId: {
        ...state.threadsByProjectId,
        [projectId]: (state.threadsByProjectId[projectId] ?? []).map(thread =>
          thread.id === threadId && updated.naming_revision >= thread.naming_revision ? updated : thread),
      },
    }));
    if (action === 'generate') {
      if (get().pendingNamingOperationByThreadId[threadId]) return;
      const marker = newClientIdentity('rename');
      set(state => ({ pendingNamingOperationByThreadId: { ...state.pendingNamingOperationByThreadId, [threadId]: marker } }));
      try {
        await generateNameCommand(projectId, threadId, previousTitle, apply);
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
    const response = await threadsApi.naming(threadId, operationId, action, title);
    const stream = streamStateByProject.get(projectId);
    if (stream && stream.epoch !== response.stream_epoch) return;
    apply(response.thread);
    if (action === 'manual') upsertCommand(threadId, {
      id: `rename:${operationId}`, type: 'command', kind: 'rename', status: 'completed',
      title: response.thread.title, content: `${previousTitle || '新会话'} → ${response.thread.title}`,
      method: 'manual', timestamp: Date.now(),
    });
  },

	applyThreadSnapshot: (projectId, epoch, sequence, threads) => {
		const current = streamStateByProject.get(projectId);
		if (current?.epoch === epoch && sequence < current.sequence) return;
		const epochChanged = Boolean(current && current.epoch !== epoch);
		streamStateByProject.set(projectId, { epoch, sequence });
		set((state) => {
			const valid = new Set(threads.map((thread) => thread.id));
			const previousThreadIds = new Set((state.threadsByProjectId[projectId] || []).map((thread) => thread.id));
			const open = (state.openThreadIdsByProjectId[projectId] || []).filter((id) => valid.has(id));
			const active = state.activeThreadIdByProjectId[projectId];
			const pending = { ...state.pendingNamingOperationByThreadId };
			if (epochChanged) {
				for (const threadId of new Set([...previousThreadIds, ...valid])) delete pending[threadId];
			}
			return {
				threadsByProjectId: { ...state.threadsByProjectId, [projectId]: threads },
				openThreadIdsByProjectId: { ...state.openThreadIdsByProjectId, [projectId]: open.length > 0 ? open : threads.map((thread) => thread.id) },
				activeThreadIdByProjectId: { ...state.activeThreadIdByProjectId, [projectId]: active && valid.has(active) ? active : threads[0]?.id ?? null },
				pendingNamingOperationByThreadId: pending,
			};
		});
	},

	applyThreadNamingUpdate: (projectId, epoch, sequence, updated) => {
		const current = streamStateByProject.get(projectId);
		if (!current || current.epoch !== epoch || sequence <= current.sequence) return;
		streamStateByProject.set(projectId, { epoch, sequence });
		set((state) => ({
			threadsByProjectId: {
				...state.threadsByProjectId,
				[projectId]: (state.threadsByProjectId[projectId] || []).map((thread) =>
					thread.id === updated.id && updated.naming_revision >= thread.naming_revision ? updated : thread),
			},
		}));
	},

	applyThreadNamingResult: (projectId, epoch, sequence, data) => {
		const current = streamStateByProject.get(projectId);
		if (!current || current.epoch !== epoch || sequence <= current.sequence) return;
		streamStateByProject.set(projectId, { epoch, sequence });
		const threadId = typeof data.thread_id === 'string' ? data.thread_id : '';
		const operationId = typeof data.operation_id === 'string' ? data.operation_id : '';
		if (!threadId || !operationId || get().pendingNamingOperationByThreadId[threadId] !== operationId) return;
		set((state) => {
			const pending = { ...state.pendingNamingOperationByThreadId };
			delete pending[threadId];
			return { pendingNamingOperationByThreadId: pending };
		});
		if (data.outcome === 'failed') showGlobalError(typeof data.error === 'string' ? data.error : '自动命名失败，请稍后重试');
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
		closeThreadEvents(oldProjectId);
		ensureThreadEvents(newProjectId);
  },

  dropProject: (projectId) => {
		closeThreadEvents(projectId);
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
	  const pending = { ...state.pendingNamingOperationByThreadId };
	  for (const threadId of ids) delete pending[threadId];
      return {
        threadsByProjectId: drop(state.threadsByProjectId),
        openThreadIdsByProjectId: drop(state.openThreadIdsByProjectId),
        activeThreadIdByProjectId: drop(state.activeThreadIdByProjectId),
		pendingNamingOperationByThreadId: pending,
		renamePanelTarget: state.renamePanelTarget?.projectId === projectId ? null : state.renamePanelTarget,
      };
    });
  },
}));
