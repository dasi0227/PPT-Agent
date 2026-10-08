import { performCommand } from './commandRuntime';
import type { CommandTimelineItem } from '../features/agent/eventReducer';
import { contextCompactionTimelineItem } from '../features/agent/eventReducer';
import { create } from 'zustand';
import { threadsApi } from '../api/threads';
import type { ContextWindowSnapshot, SSEEvent } from '../api/types';

interface ContextWindowSession {
  snapshot: ContextWindowSnapshot | null;
  loading: boolean;
  compacting: boolean;
}

interface ContextWindowState {
  sessions: Record<string, ContextWindowSession>;
  load: (threadId: string, modelProfileName: string) => Promise<void>;
  compact: (threadId: string, commandId?: string) => Promise<boolean>;
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
    const previousSnapshot = get().sessions[threadId]?.snapshot;
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
          [threadId]: {
            ...(state.sessions[threadId] ?? emptySession()),
            snapshot: state.sessions[threadId]?.snapshot === (previousSnapshot ?? null)
              ? snapshot : state.sessions[threadId]?.snapshot ?? snapshot,
            loading: false,
          },
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
  compact: async (threadId, commandId) => {
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
    const id = commandId ?? `compact:${crypto.randomUUID()}`;
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
        (signal, onProgress) => threadsApi.compact(threadId, signal, onProgress, id),
        (result) => {
          set((state) => ({
            sessions: {
              ...state.sessions,
              [threadId]: { snapshot: result.snapshot, loading: false, compacting: false },
            },
          }));
          return { ...contextCompactionTimelineItem(result.compaction), id };
        },
        () => {
          void get().compact(threadId, id);
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
          compacting: snapshot.status === 'compacting' || state.sessions[threadId]?.compacting === true,
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

// Runtime compaction is projected directly from run events, including replay.
export function receiveCompactionEvent(threadId: string, event: SSEEvent): void {
  if (['run.failed', 'run.error', 'run.canceled', 'run.completed'].includes(event.event)) {
    useContextWindowStore.setState((state) => ({
      sessions: {
        ...state.sessions,
        [threadId]: { ...(state.sessions[threadId] ?? emptySession()), compacting: false },
      },
    }));
  }
}
