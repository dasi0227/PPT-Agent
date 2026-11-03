import { APIError, RequestCanceledError } from '../api/client';
import { runsApi, type RunAction } from '../api/runs';
import { threadsApi } from '../api/threads';
import type { CreateRunRequest, RunCancelReason, ScopeExpansionRequest } from '../api/types';
import { newClientIdentity } from '../lib/clientIdentity';
import { presentSession, UNLOADED_SESSION, type RunSession } from '../features/agent/sessionPresentation';
import { sessionCanWrite, useSessionStore, type SessionState } from './sessionStore';
import { useComposerStore } from './composerStore';
import { showGlobalError, showGlobalWarning } from './toastStore';
import { isProjectDeleted, isThreadDeleted } from './deletedTargets';

export type { PlanState } from '../api/types';
export type { RunSession, RunStatus } from '../features/agent/sessionPresentation';
export { UNLOADED_SESSION };

interface InteractionVersion { scene_revision: string; request_revision: string }

// Keep an ambiguous request unchanged across user retries. These are local
// action identities, never an alternative source of persisted Run state.
const cancelRequests = new Map<string, { threadId: string; action: RunAction }>();

function current(threadId: string, scene?: string): SessionState & { sceneRevision: string } {
  const state = useSessionStore.getState().sessions[threadId];
  if (isThreadDeleted(threadId) || !state || isProjectDeleted(state.projectId)) throw new RequestCanceledError();
  if (!sessionCanWrite(state)) throw state.error ?? new Error('会话加载失败，请重试');
  if (scene !== undefined && state.sceneRevision !== scene) throw new APIError(409, 'SCENE_CHANGED', 'SCENE_CHANGED');
  return state;
}

function interaction(threadId: string, runId: string, id: string, version: InteractionVersion): void {
  const state = current(threadId, version.scene_revision);
  const row = state.interactionsById[id];
  if (!row || row.run_id !== runId || row.request_revision !== version.request_revision)
    throw new APIError(409, 'VERSION_CONFLICT', 'VERSION_CONFLICT');
}

function report(error: unknown): void {
  if (!(error instanceof RequestCanceledError)) showGlobalError(error instanceof Error ? error.message : '请求失败，请重试');
}

async function submit(threadId: string, action: () => Promise<unknown>): Promise<boolean> {
  try {
    await action();
    // A receipt confirms this action, but cannot skip earlier committed batches.
    await useSessionStore.getState().reconnect(threadId);
    return true;
  } catch (error) {
    report(error);
    if (error instanceof APIError && ['SCENE_CHANGED', 'VERSION_CONFLICT'].includes(error.code ?? ''))
      void useSessionStore.getState().retry(threadId);
    return false;
  }
}

const actions = {
  async clearPlan(threadId: string): Promise<void> {
    const state = current(threadId);
    await threadsApi.clearPlan(threadId, state.sceneRevision, state.thread!.plan_revision);
    await useSessionStore.getState().reconnect(threadId);
  },
  async createRun(threadId: string, request: CreateRunRequest, projectId?: string): Promise<'created' | 'canceled' | 'failed'> {
    if (isThreadDeleted(threadId)) return 'canceled';
    const store = useSessionStore.getState();
    const project = projectId ?? store.sessions[threadId]?.projectId;
    if (!project || isProjectDeleted(project)) return 'canceled';
    await store.ensure(threadId, project);
    const clientMessageId = request.client_request_id ?? newClientIdentity('req');
    const payload = { ...request, client_request_id: clientMessageId };
    const createdAt = Date.now();
    let scene: string | undefined;
    try {
      const state = current(threadId);
      scene = state.sceneRevision;
      store.putOptimisticInput(threadId, project, { clientMessageId, text: payload.instruction, request: payload,
        createdAt, status: 'sending', sceneRevision: scene });
      const run = await runsApi.create(threadId, { ...payload, scene_revision: scene });
      if (isThreadDeleted(threadId) || isProjectDeleted(project)) return 'canceled';
      if (useSessionStore.getState().sessions[threadId]?.sceneRevision !== scene) return 'canceled';
      store.putOptimisticInput(threadId, project, { clientMessageId, text: payload.instruction, request: payload,
        createdAt, status: 'accepted', runId: run.id, sceneRevision: scene });
      if (run.dropped_mentioned_slide_ids?.length) showGlobalWarning(
        run.dropped_mentioned_slide_ids.length === payload.mentioned_slide_ids?.length
          ? '所点页面均不存在，已按未点名页面继续执行'
          : `已忽略 ${run.dropped_mentioned_slide_ids.length} 个不存在的页面，其余页面继续执行`);
      if (run.model) useComposerStore.getState().setModelProfileName(run.model);
      await store.reconnect(threadId);
      return 'created';
    } catch (error) {
      if (scene && !isThreadDeleted(threadId)) store.putOptimisticInput(threadId, project, {
        clientMessageId, text: payload.instruction, request: payload, createdAt, status: 'error',
        sceneRevision: scene, error: error instanceof Error ? error : new Error(String(error)),
      });
      report(error);
      return error instanceof RequestCanceledError ? 'canceled' : 'failed';
    }
  },
  answerQuestion(threadId: string, runId: string, replyTo: string, content: string, version: InteractionVersion): Promise<boolean> {
    return submit(threadId, () => { interaction(threadId, runId, replyTo, version); return runsApi.submitInput(runId, { reply_to: replyTo, content, ...version }); });
  },
  answerCommandPermission(threadId: string, runId: string, interactionId: string, callId: string, commandHash: string,
    decision: 'allow_once' | 'deny', feedback: string | undefined, version: InteractionVersion): Promise<boolean> {
    return submit(threadId, () => {
      interaction(threadId, runId, interactionId, version);
      return runsApi.submitCommandPermission(runId, { interaction_id: interactionId, call_id: callId, command_hash: commandHash, decision, feedback, ...version });
    });
  },
  answerScopeExpansion(threadId: string, runId: string, payload: ScopeExpansionRequest & InteractionVersion): Promise<boolean> {
    return submit(threadId, () => { interaction(threadId, runId, payload.interaction_id, payload); return runsApi.submitScopeExpansion(runId, payload); });
  },
  async cancelRun(threadId: string, runId: string, reason: RunCancelReason = 'user_requested'): Promise<boolean> {
    let ended = false;
    const accepted = await submit(threadId, async () => {
      const state = current(threadId);
      const run = state.runsById[runId];
      if (!run) throw new APIError(409, 'VERSION_CONFLICT', 'VERSION_CONFLICT');
      const key = `${runId}:${reason}`;
      const generation = state.generation;
      let pending = cancelRequests.get(key);
      if (pending && pending.action.scene_revision !== state.sceneRevision) {
        cancelRequests.delete(key);
        throw new APIError(409, 'SCENE_CHANGED', 'SCENE_CHANGED');
      }
      if (!pending) {
        pending = { threadId, action: {
          scene_revision: state.sceneRevision, entity_revision: run.entity_revision, request_key: newClientIdentity('cancel'),
        } };
        cancelRequests.set(key, pending);
      }
      try {
        const result = await runsApi.cancel(runId, reason, pending.action);
        if (current(threadId, pending.action.scene_revision).generation !== generation) throw new RequestCanceledError();
        ended = ['done', 'failed', 'canceled'].includes(result.run.status);
        if (cancelRequests.get(key) === pending) cancelRequests.delete(key);
      } catch (error) {
        if (error instanceof APIError && error.status < 500 && error.status !== 408 && error.status !== 429 && cancelRequests.get(key) === pending) cancelRequests.delete(key);
        throw error;
      }
    });
    return accepted && (reason !== 'superseded' || ended);
  },
  async steerRun(threadId: string, runId: string, content: string, clientMessageId: string,
    attachmentIds: string[] = [], domSelections: import('../api/types').DOMSelection[] = [], referenceOrder: import('../api/types').ReferenceOrderItem[] = []): Promise<boolean> {
    let state: SessionState & { sceneRevision: string };
    try {
      state = current(threadId);
      if (state.thread?.active_run_id !== runId) return false;
    } catch (error) { report(error); return false; }
    const store = useSessionStore.getState();
    const input = { clientMessageId, text: content, createdAt: Date.now(), runId, sceneRevision: state.sceneRevision };
    store.putOptimisticInput(threadId, state.projectId, { ...input, status: 'sending' });
    const accepted = await submit(threadId, () => runsApi.steer(runId, {
      scene_revision: state.sceneRevision, expected_run_id: runId, client_message_id: clientMessageId,
      content, attachment_ids: attachmentIds, dom_selections: domSelections, reference_order: referenceOrder,
    }));
    store.putOptimisticInput(threadId, state.projectId, { ...input, status: accepted ? 'accepted' : 'error' });
    return accepted;
  },
  dropSessions(threadIds: string[]): void {
    for (const [key, pending] of cancelRequests) if (threadIds.includes(pending.threadId)) cancelRequests.delete(key);
    useSessionStore.getState().drop(threadIds);
  },
};

type RunStore = typeof actions & { sessions: Record<string, RunSession> };
const roots = new WeakMap<Record<string, SessionState>, RunStore>();
function presentRoot(sessions: Record<string, SessionState>): RunStore {
  const cached = roots.get(sessions);
  if (cached) return cached;
  const value = { ...actions, sessions: Object.fromEntries(Object.entries(sessions).map(([id, state]) => [id, presentSession(state)])) };
  roots.set(sessions, value);
  return value;
}

// This is a selector/action facade, not a second mutable run-state store.
export const useRunStore = Object.assign(
  function useRunView<T = RunStore>(selector: (value: RunStore) => T = value => value as unknown as T): T {
    return useSessionStore(state => selector(presentRoot(state.sessions)));
  },
  { getState: (): RunStore => presentRoot(useSessionStore.getState().sessions) },
);
