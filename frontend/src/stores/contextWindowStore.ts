import { RequestCanceledError } from '../api/client';
import { commandActive, performCommand } from './commandRuntime';
import type { CommandTimelineItem } from '../features/agent/eventReducer';
import { contextCompactionTimelineItem } from '../features/agent/eventReducer';
import { notifyModelFallback } from '../lib/modelExecution';
import { create } from 'zustand';
import { threadsApi } from '../api/threads';
import type { ContextCompaction, ContextWindowSnapshot, SSEEvent } from '../api/types';

interface ContextWindowSession {
  snapshot: ContextWindowSnapshot | null;
  loading: boolean;
  compacting: boolean;
}

interface ContextWindowState {
  sessions: Record<string, ContextWindowSession>;
  load: (threadId: string, modelProfileName: string) => Promise<void>;
  compact: (threadId: string) => Promise<boolean>;
  update: (threadId: string, snapshot: ContextWindowSnapshot) => void;
  drop: (threadId: string) => void;
}

const emptySession = (): ContextWindowSession => ({
  snapshot: null,
  loading: false,
  compacting: false,
});

export const useContextWindowStore = create<ContextWindowState>((set, get) => ({
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
          [threadId]: { ...(state.sessions[threadId] ?? emptySession()), snapshot, loading: false },
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
  compact: async (threadId) => {
    const current = get().sessions[threadId];
    if (current?.compacting || !current?.snapshot
      || current.snapshot.compact_threshold_tokens <= 0
      || current.snapshot.compactable_tokens < current.snapshot.compact_threshold_tokens) return false;
    set((state) => ({
      sessions: {
        ...state.sessions,
        [threadId]: { ...(state.sessions[threadId] ?? emptySession()), compacting: true },
      },
    }));
    const id = `compact:${threadId}`;
    const initial: CommandTimelineItem = {
      id,
      type: 'command',
      kind: 'compact',
      title: '压缩上下文',
      status: 'loading',
      timestamp: Date.now(),
    };
    try {
      return await performCommand(
        threadId,
        initial,
        (signal, onProgress) => threadsApi.compact(threadId, signal, onProgress),
        (result) => {
          notifyModelFallback(result.model_execution, '上下文压缩');
          set((state) => ({
            sessions: {
              ...state.sessions,
              [threadId]: { snapshot: result.snapshot, loading: false, compacting: false },
            },
          }));
          return contextCompactionTimelineItem(result.compaction);
        },
        () => {
          void get().compact(threadId);
        },
      );
    } finally {
      set((state) => ({
        sessions: {
          ...state.sessions,
          [threadId]: { ...(state.sessions[threadId] ?? emptySession()), compacting: false },
        },
      }));
    }
  },

  update: (threadId, snapshot) =>
    set((state) => ({
      sessions: {
        ...state.sessions,
        [threadId]: {
          snapshot,
          loading: false,
          compacting: snapshot.status === 'compacting' || commandActive(`compact:${threadId}`),
        },
      },
    })),
  drop: (threadId) =>
    set((state) => {
      const sessions = { ...state.sessions };
      delete sessions[threadId];
      return { sessions };
    }),
}));

// Automatic compaction belongs to the active run and is stopped with that run.
const automatic = new Map<
  string,
  {
    id: string;
    phase: (value: number) => void;
    resolve: (value: ContextCompaction) => void;
    reject: (error: Error) => void;
  }
>();
export function receiveCompactionEvent(threadId: string, event: SSEEvent): boolean {
  if (event.event === 'context.window.updated' && event.data.compaction) {
    const progress = event.data.compaction;
    let pending = automatic.get(threadId);
    if (pending?.id !== progress.id) {
      pending?.reject(new Error('压缩已被替代'));
      const result = new Promise<ContextCompaction>((resolve, reject) => {
        pending = { id: progress.id, phase: () => {}, resolve, reject };
        automatic.set(threadId, pending);
      });
      const job = pending!;
      const initial: CommandTimelineItem = {
        id: `compact:${progress.id}`,
        type: 'command',
        kind: 'compact',
        title: '压缩上下文',
        status: 'loading',
        method: 'auto',
        cancellable: false,
        timestamp: Date.parse(event.data.occurred_at),
      };
      void performCommand(
        threadId,
        initial,
        (_signal, phase) => {
          job.phase = phase;
          return result;
        },
        contextCompactionTimelineItem,
        () => {
          void useContextWindowStore.getState().compact(threadId);
        },
      );
    }
    pending!.phase(progress.phase);
  } else if (event.event === 'context.compacted') {
    const pending = automatic.get(threadId);
    if (pending) {
      automatic.delete(threadId);
      pending.resolve(event.data.compaction);
      return true;
    }
  } else if (['run.failed', 'run.error', 'run.canceled', 'run.completed'].includes(event.event)) {
    automatic
      .get(threadId)
      ?.reject(
        event.event === 'run.canceled' ? new RequestCanceledError() : new Error('运行已结束'),
      );
    useContextWindowStore.setState((state) => ({
      sessions: {
        ...state.sessions,
        [threadId]: { ...(state.sessions[threadId] ?? emptySession()), compacting: false },
      },
    }));
    automatic.delete(threadId);
  }
  return false;
}
