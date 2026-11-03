import { create } from 'zustand';
import { threadsApi } from '../api/threads';
import type { ContextWindowSnapshot } from '../api/types';
import type { SessionContextWindow } from '../api/session';
import { performCommand } from './commandRuntime';
import { isThreadDeleted } from './deletedTargets';
import { sessionCanWrite, useSessionStore, type SessionState } from './sessionStore';

// Only on-demand token details and local request state live here. The header's
// current usage and running compaction status come from the session snapshot.
interface ContextWindowSession {
  snapshot: ContextWindowSnapshot | null;
  model: string;
  sceneRevision: string | null;
  generation: number;
  contextRevision: string | null;
  planRevision: string | null;
  loading: boolean;
  compacting: boolean;
  error: Error | null;
}
interface ContextWindowState {
  sessions: Record<string, ContextWindowSession>;
  load: (threadId: string, model: string) => Promise<void>;
  compact: (threadId: string, commandId?: string) => Promise<boolean>;
  drop: (threadId: string) => void;
}
const requests = new Map<string, { controller: AbortController; promise: Promise<void>; key: string }>();
const emptySession = (): ContextWindowSession => ({ snapshot: null, model: '', sceneRevision: null,
  generation: 0, contextRevision: null, planRevision: null, loading: false, compacting: false, error: null });

export function selectContextWindow(source: SessionState | undefined, detail: ContextWindowSession | undefined,
  model?: string): SessionContextWindow | null {
  if (!source) return null;
  if (detail?.snapshot && detail.sceneRevision === source.sceneRevision && detail.generation === source.generation
    && detail.contextRevision === source.thread?.context_revision && detail.planRevision === source.thread?.plan_revision && (model === undefined || model === detail.model)) return detail.snapshot;
  return source.contextWindow;
}

export const useContextWindowStore = create<ContextWindowState>((set, get) => ({
  sessions: {},
  load: (threadId, model) => {
    const source = useSessionStore.getState().sessions[threadId];
    if (isThreadDeleted(threadId) || !sessionCanWrite(source)) return Promise.resolve();
    const key = `${source.generation}:${source.sceneRevision}:${source.thread!.context_revision}:${source.thread!.plan_revision}:${model}`;
    const pending = requests.get(threadId);
    if (pending?.key === key) return pending.promise;
    pending?.controller.abort();
    const controller = new AbortController();
    const request = { controller, key, promise: Promise.resolve() };
    requests.set(threadId, request);
    set(state => ({ sessions: { ...state.sessions, [threadId]: {
      ...(state.sessions[threadId] ?? emptySession()), loading: true, error: null,
    } } }));
    request.promise = (async () => {
      try {
        const snapshot = await threadsApi.contextWindow(threadId, model, source.sceneRevision, controller.signal);
        const now = useSessionStore.getState().sessions[threadId];
        if (requests.get(threadId) !== request || isThreadDeleted(threadId) || !sessionCanWrite(now)
          || now.generation !== source.generation || now.sceneRevision !== source.sceneRevision || now.thread?.context_revision !== source.thread?.context_revision || now.thread?.plan_revision !== source.thread?.plan_revision) return;
        set(state => ({ sessions: { ...state.sessions, [threadId]: {
          ...(state.sessions[threadId] ?? emptySession()), snapshot, model, loading: false, error: null,
          sceneRevision: source.sceneRevision, generation: source.generation, contextRevision: source.thread!.context_revision, planRevision: source.thread!.plan_revision,
        } } }));
      } catch (error) {
        if (requests.get(threadId) === request && !controller.signal.aborted && !isThreadDeleted(threadId))
          set(state => ({ sessions: { ...state.sessions, [threadId]: {
            ...(state.sessions[threadId] ?? emptySession()), error: error instanceof Error ? error : new Error(String(error)),
          } } }));
      } finally {
        if (requests.get(threadId) === request) {
          requests.delete(threadId);
          set(state => state.sessions[threadId] ? { sessions: { ...state.sessions,
            [threadId]: { ...state.sessions[threadId], loading: false },
          } } : state);
        }
      }
    })();
    return request.promise;
  },
  compact: async (threadId, commandId) => {
    const source = useSessionStore.getState().sessions[threadId];
    const local = get().sessions[threadId];
    const snapshot = selectContextWindow(source, local);
    if (isThreadDeleted(threadId) || !sessionCanWrite(source) || local?.compacting || !snapshot
      || snapshot.compact_threshold_tokens <= 0 || snapshot.compactable_tokens < snapshot.compact_threshold_tokens) return false;
    const id = commandId ?? `compact:${crypto.randomUUID()}`;
    set(state => ({ sessions: { ...state.sessions, [threadId]: {
      ...(state.sessions[threadId] ?? emptySession()), compacting: true,
    } } }));
    try {
      return await performCommand(threadId, id,
        signal => threadsApi.compact(threadId, signal, undefined, id),
        () => {}, () => { void get().compact(threadId, id); });
    } finally {
      if (!isThreadDeleted(threadId)) set(state => ({ sessions: { ...state.sessions, [threadId]: {
        ...(state.sessions[threadId] ?? emptySession()), compacting: false,
      } } }));
    }
  },
  drop: threadId => {
    requests.get(threadId)?.controller.abort();
    requests.delete(threadId);
    set(state => { const sessions = { ...state.sessions }; delete sessions[threadId]; return { sessions }; });
  },
}));
