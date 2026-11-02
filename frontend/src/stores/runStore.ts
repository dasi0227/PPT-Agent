import { isResourceEditTool } from '../api/resourceTools';
import { reduceRunClock, runElapsed, type RunClock } from '../features/agent/runClock';
import { mergeRunTokenUsage, reduceRunTokenUsage, type RunTokenUsageByRun } from '../features/agent/runTokenUsage';
import { reconcileRunTimeline, runStopped, settleTimelineActivities } from '../features/agent/runOutcome';
import { create } from 'zustand';
import { APIError, currentHistoryEpoch, RequestCanceledError } from '../api/client';
import { runsApi } from '../api/runs';
import { subscribeRunEvents } from '../api/sse';
import { threadsApi } from '../api/threads';
import {
  CreateRunRequest,
  CreateRunScopeInput,
  PlanState,
  QuestionAnswer,
  QuestionFieldAnswer,
  Run,
  RunCancelReason,
  RunMode,
  RunProgress,
  RunScope,
  ResourceEditApproval,
  ScopeExpansionRequest,
  SSEEvent,
} from '../api/types';
import {
  TimelineItem,
  reducePlan,
  reduceRunProgress,
  reduceSSEEvent,
  settleRunActivities,
} from '../features/agent/eventReducer';
import { reduceNextInputSuggestions, type NextInputSuggestionsState } from '../features/agent/nextInputSuggestions';
import {
  hydrateRunFromHistory,
  type HistoryEntry,
  type HistorySessionState,
} from '../features/agent/historyHydrator';
import { useProjectStore } from './projectStore';
import { useComposerStore } from './composerStore';
import { newClientIdentity } from '../lib/clientIdentity';
import { showGlobalError, showGlobalWarning } from './toastStore';
import { receiveCompactionEvent, useContextWindowStore } from './contextWindowStore';
import { isProjectDeleted, isThreadDeleted } from './deletedTargets';

export type { PlanState } from '../api/types';

export type RunStatus = 'idle' | 'creating' | 'running' | 'waiting' | 'paused' | 'recovering' | 'canceling' | 'done' | 'error' | 'canceled';
export type StreamStatus = 'idle' | 'connecting' | 'open' | 'reconnecting' | 'closed';

export interface RunSession {
  tokenUsageByRun: RunTokenUsageByRun;
  stoppedRuns?: Record<string, { run: Run; sequence: number }>;
  runClock?: RunClock;
  activeRunId: string | null;
  activeRunModel?: string | null;
  projectId?: string | null;
  status: RunStatus;
  streamStatus?: StreamStatus;
  scope: RunScope | CreateRunScopeInput;
  mode: RunMode;
  timelineItems: TimelineItem[];
  pendingQuestion: { id: string } | null;
  progress: RunProgress | null;
  eventSourceClose: (() => void) | null;
  plan: PlanState | null;
  planSequence?: number;
  // Last task event, excluding accounting. The transport owns its thread cursor.
  lastEventId?: string;
  processedEventIds?: string[];
  originalRequest?: CreateRunRequest;
  nextInputSuggestions: NextInputSuggestionsState | null;
}

export const IDLE_SESSION: RunSession = Object.freeze<RunSession>({
  tokenUsageByRun: {},
  activeRunId: null,
  activeRunModel: null,
  projectId: null,
  status: 'idle',
  streamStatus: 'idle',
  scope: { selection: { kind: 'current_page' } },
  mode: 'create',
  timelineItems: [],
  pendingQuestion: null,
  progress: null,
  eventSourceClose: null,
  plan: null,
  processedEventIds: [],
  nextInputSuggestions: null,
});

interface PersistedActiveRun {
  runId: string;
  threadId: string;
  canceling?: boolean;
}

const ACTIVE_RUNS_KEY = 'ppt-agent-active-runs-v1';
const CANCEL_RECONCILIATION_DELAYS_MS = [3_000, 5_000, 10_000] as const;

interface CancelReconciliation {
  runId: string;
  timer?: ReturnType<typeof setTimeout>;
}

function freshSession(overrides: Partial<RunSession> = {}): RunSession {
  return {
    ...IDLE_SESSION,
    tokenUsageByRun: {},
    processedEventIds: [],
    ...overrides,
  };
}

function readPersistedRuns(): Record<string, PersistedActiveRun> {
  if (typeof sessionStorage === 'undefined') return {};
  try {
    const raw = sessionStorage.getItem(ACTIVE_RUNS_KEY);
    return raw ? JSON.parse(raw) as Record<string, PersistedActiveRun> : {};
  } catch {
    return {};
  }
}

function writePersistedRun(run: PersistedActiveRun): void {
  if (isThreadDeleted(run.threadId)) return;
  if (typeof sessionStorage === 'undefined') return;
  const records = readPersistedRuns();
  records[run.threadId] = run;
  sessionStorage.setItem(ACTIVE_RUNS_KEY, JSON.stringify(records));
}

function removePersistedRun(threadId: string): void {
  if (typeof sessionStorage === 'undefined') return;
  const records = readPersistedRuns();
  delete records[threadId];
  if (Object.keys(records).length === 0) sessionStorage.removeItem(ACTIVE_RUNS_KEY);
  else sessionStorage.setItem(ACTIVE_RUNS_KEY, JSON.stringify(records));
}

function errorMessage(error: unknown): { message: string; code?: string; requestId?: string; retryable?: boolean } {
  if (error instanceof APIError) {
    return {
      message: error.message,
      code: error.code,
      requestId: error.requestId,
      retryable: error.retryable,
    };
  }
  return {
    message: error instanceof Error ? error.message : '请求失败，请重试',
    retryable: typeof error === 'object' && error !== null && 'retryable' in error
      ? (error as { retryable?: unknown }).retryable === true
      : false,
  };
}

function localizedErrorMessage(message: string, fallback: string): string {
  return /[\u3400-\u9fff]/u.test(message) ? message : fallback;
}

function parseQuestionAnswer(content: string): QuestionAnswer | null {
  try {
    const parsed = JSON.parse(content) as Partial<QuestionAnswer>;
    if (!parsed || typeof parsed !== 'object' || !Array.isArray(parsed.answers)) return null;
    if (Object.keys(parsed).some((key) => key !== 'answers')) return null;
    const answers = parsed.answers
      .map((answer): QuestionFieldAnswer | null => {
        if (!answer || typeof answer !== 'object') return null;
        const candidate = answer as Partial<QuestionFieldAnswer>;
        if (typeof candidate.question_id !== 'string') return null;
        if (Object.keys(candidate).some((key) => !['question_id', 'selected_option_id', 'custom_text'].includes(key))) return null;
        return {
          question_id: candidate.question_id,
          ...(typeof candidate.selected_option_id === 'string' ? { selected_option_id: candidate.selected_option_id } : {}),
          ...(typeof candidate.custom_text === 'string' ? { custom_text: candidate.custom_text } : {}),
        };
      })
      .filter((answer): answer is QuestionFieldAnswer => Boolean(answer));
    return answers.length > 0 ? { answers } : null;
  } catch {
    return null;
  }
}

function withPendingCancellation(status: RunStatus, requested: boolean): RunStatus {
  return requested && (status === 'running' || status === 'waiting' || status === 'recovering') ? 'canceling' : status;
}

function sessionStatusForRun(run: Pick<Run, 'status' | 'cancel_requested_at'>, canceling = false): RunStatus {
  if (run.status === 'failed') return 'error';
  const status = run.status === 'pending' ? 'running' : run.status;
  return withPendingCancellation(status, Boolean(run.cancel_requested_at) || canceling);
}

function isTerminalRunStatus(status: string): status is 'done' | 'failed' | 'canceled' {
  return status === 'done' || status === 'failed' || status === 'canceled';
}

function mergeAuthoritativeTimeline(current: TimelineItem[], authoritative: TimelineItem[], preserveRunId?: string): TimelineItem[] {
  // At the same task sequence, keep local approvals/resumption while filling
  // missing history. Independent commands and user turns still reconcile.
  if (preserveRunId) {
    authoritative = authoritative.filter(item => item.runId !== preserveRunId
      || (item.type !== 'terminal_notice' && item.type !== 'final')
      || !current.some(next => next.type === 'run_lifecycle' && next.runId === preserveRunId && next.timestamp > item.timestamp));
  }
  const authoritativeById = new Map(authoritative.map((item) => [item.id, item]));
  current = current.filter(item => {
    if (item.type !== 'terminal_notice' && item.type !== 'final') return true;
    return authoritativeById.has(item.id) || !authoritative.some(next => next.type === 'run_lifecycle'
      && next.runId === item.runId && next.timestamp > item.timestamp);
  });
  const transferredIds = new Set(authoritative.flatMap(item => item.type === 'user_turn'
    ? (item.sourceMessageIds ?? []).map(id => `steering_${id}`) : []));
  current = current.filter(item => !transferredIds.has(item.id) || authoritativeById.has(item.id));
  const currentIds = new Set(current.map((item) => item.id));
  const authoritativeOriginalTurns = new Map(
    authoritative
      .filter((item) => item.type === 'user_turn' && item.runId && item.scope && item.mode)
      .map((item) => [item.runId!, item]),
  );
  const mergedOriginalRunIds = new Set<string>();
  const merged = [
    ...current.map((item) => {
      const exact = authoritativeById.get(item.id);
      if (exact) return preserveRunId && item.runId === preserveRunId && item.type !== 'user_turn' ? item : exact;
      if (item.type !== 'user_turn' || !item.runId || !item.scope || !item.mode) return item;
      const original = authoritativeOriginalTurns.get(item.runId);
      if (!original) return item;
      mergedOriginalRunIds.add(item.runId);
      return { ...original, id: item.id };
    }),
    ...authoritative.filter((item) => (
      !currentIds.has(item.id)
      && !(item.type === 'user_turn' && item.runId && mergedOriginalRunIds.has(item.runId))
    )),
  ];
  // A command may arrive through the journal after later run events were
  // already rendered. Reorder authoritative slots by journal sequence while
  // retaining local drafts/progress rows and their stable IDs.
  const order = new Map(authoritative.map((item, index) => [item.id, index]));
  for (const item of merged) {
    if (item.type === 'user_turn' && item.runId && item.scope && item.mode) {
      const original = authoritativeOriginalTurns.get(item.runId);
      if (original) order.set(item.id, order.get(original.id)!);
    }
  }
  const ordered = merged.filter(item => order.has(item.id)).sort((a, b) => order.get(a.id)! - order.get(b.id)!);
  let cursor = 0;
  return merged.map(item => order.has(item.id) ? ordered[cursor++] : item);
}

function ensureTerminalTimelineItem(
  items: TimelineItem[],
  runId: string,
  status: 'done' | 'failed' | 'canceled',
): TimelineItem[] {
  const hasTerminal = items.some((item) =>
    item.runId === runId && (item.type === 'final' || item.type === 'terminal_notice'));
  if (hasTerminal) return items;
  const occurredAt = new Date().toISOString();
  if (status === 'done') {
    return reduceSSEEvent(items, {
      event: 'message.final',
      data: {
        schema_version: 6,
        run_id: runId,
        occurred_at: occurredAt,
        message_id: `${runId}:reconciled-final`,
        text: '任务已完成。',
        affected_targets: [],
      },
    });
  }
  return reduceSSEEvent(items, {
    event: status === 'canceled' ? 'run.canceled' : 'run.error',
    data: {
      schema_version: 6,
      run_id: runId,
      occurred_at: occurredAt,
      duration_ms: 0,
      affected_targets: [],
      error: status === 'failed'
        ? { code: 'RUN_FAILED', message: '任务未能完成，请重试。', retryable: true }
        : null,
      trace_id: runId,
    },
  });
}

function requestFromTimeline(
  items: TimelineItem[],
  runId?: string | null,
  model?: string | null,
): CreateRunRequest | undefined {
  const original = [...items].reverse().find((item) =>
    item.type === 'user_turn' &&
    Boolean(item.scope) &&
    Boolean(item.mode) &&
    (!runId || item.runId === runId));
  if (!original || original.type !== 'user_turn' || !original.scope || !original.mode) return undefined;
  let scope: CreateRunScopeInput;
  if ('selection' in original.scope) {
    scope = original.scope as CreateRunScopeInput;
  } else {
    const normalized = original.scope;
    const source = normalized.source;
    scope = {
      selection: source.kind === 'all_pages'
        ? { kind: 'all_pages' }
        : source.kind === 'current_page'
          ? { kind: 'current_page', current_slide_id: normalized.slide_ids[0] }
          : source.kind === 'custom_sections' && source.section_ids?.length
            ? { kind: 'custom_sections', section_ids: source.section_ids }
            : { kind: 'custom_pages', slide_ids: normalized.slide_ids },
    };
  }
  return {
    client_request_id: newClientIdentity('req'),
    ...(model ? { model } : {}),
    scope,
    mode: original.mode as RunMode,
    instruction: original.text,
    ...(original.skills?.length ? { skill_ids: original.skills.map((skill) => skill.id) } : {}),
    ...(original.components?.length ? { component_names: original.components.map((component) => component.name) } : {}),
  };
}

type CreateRunResult = 'created' | 'canceled' | 'failed';

// Plan revisions use the thread journal sequence, including explicit clears.
function newerPlan(prev: Pick<RunSession, 'planSequence'>, plan: PlanState | null, sequence = 0) {
  return sequence >= (prev.planSequence ?? 0) ? { plan, planSequence: sequence } : {};
}

interface RunStoreV2 {
  clearPlan: (threadId: string) => Promise<void>;
  sessions: Record<string, RunSession>;
  createRun: (threadId: string, payload: CreateRunRequest, projectId?: string) => Promise<CreateRunResult>;
  subscribeRun: (threadId: string, runId: string, projectId?: string) => void;
  recoverPersistedRuns: () => Promise<void>;
  syncThreadHistory: (threadId: string, entries: HistoryEntry[], projectId?: string) => boolean;
  reconcileRun: (threadId: string, runId: string, lastEventId?: string, projectId?: string) => Promise<void>;
  answerQuestion: (threadId: string, runId: string, replyTo: string, content: string) => Promise<boolean>;
  answerCommandPermission: (
    threadId: string,
    runId: string,
    interactionId: string,
    callId: string,
    commandHash: string,
    decision: 'allow_once' | 'deny',
    feedback?: string,
  ) => Promise<boolean>;
  answerScopeExpansion: (threadId: string, runId: string, payload: ScopeExpansionRequest) => Promise<boolean>;
  syncResourceApprovalDraft: (record: ResourceEditApproval) => void;
  cancelRun: (threadId: string, runId: string, reason?: RunCancelReason) => Promise<boolean>;
	steerRun: (threadId: string, runId: string, content: string, clientMessageId: string, attachmentIds?: string[], domSelections?: import('../api/types').DOMSelection[], referenceOrder?: import('../api/types').ReferenceOrderItem[]) => Promise<boolean>;
  closeSessions: (threadIds: string[]) => void;
  dropSessions: (threadIds: string[]) => void;
  hydrateTimeline: (
    threadId: string,
    items: TimelineItem[],
    plan?: PlanState | null,
    session?: HistorySessionState,
    lastEventId?: string,
    run?: Run | null,
  ) => void;
}

function endPausedRun(items: TimelineItem[], runId: string): TimelineItem[] {
  return reduceSSEEvent(items, {
    event: 'run.canceled',
    data: {
      schema_version: 6,
      run_id: runId,
      occurred_at: new Date().toISOString(),
      duration_ms: 0,
      affected_targets: [],
      error: null,
      trace_id: runId,
      reason: 'superseded',
    },
  });
}

// Every ingress (live, HTTP reconciliation, replay, optimistic updates) obeys
// the same stopped-state invariant. Journal replay cannot resurrect old loaders.
function normalizeSession(session: RunSession): RunSession {
  let timelineItems = session.timelineItems;
  for (const { run } of Object.values(session.stoppedRuns ?? {})) {
    timelineItems = reconcileRunTimeline(timelineItems, run);
  }
  timelineItems = settleTimelineActivities(timelineItems);
  const snapshot = session.activeRunId ? session.stoppedRuns?.[session.activeRunId]?.run : undefined;
  const notice = timelineItems.find(item => item.type === 'terminal_notice' && item.runId === session.activeRunId);
  const status = snapshot ? sessionStatusForRun(snapshot) : session.status;
  const stopped = ['paused', 'done', 'error', 'canceled'].includes(status);
  if (stopped && session.activeRunId) timelineItems = settleRunActivities(timelineItems, session.activeRunId,
    notice?.type === 'terminal_notice' && notice.reason === 'user_requested' ? 'canceled' : 'interrupted');
  const timestamp = snapshot ? (snapshot.paused_at || snapshot.updated_at || 0) * 1000 : notice?.timestamp;
  return {
    ...session, timelineItems, status,
    progress: reduceRunProgress(session.progress, status),
    ...(stopped ? {
      pendingQuestion: null, streamStatus: 'closed' as const,
      runClock: session.runClock ? { elapsedMs: snapshot?.active_duration_ms ?? runElapsed(session.runClock, timestamp || Date.now()), runningSince: null } : undefined,
    } : {}),
  };
}

export const useRunStore = create<RunStoreV2>((set, get) => {
  const cancelReconciliations = new Map<string, CancelReconciliation>();
  const reconcilingRuns = new Set<string>();

  const patchSession = (threadId: string, patch: Partial<RunSession>) => {
    if (isThreadDeleted(threadId)) return;
    set((state) => {
      const prev = state.sessions[threadId] ?? freshSession();
      return { sessions: { ...state.sessions, [threadId]: normalizeSession({ ...prev, ...patch }) } };
    });
  };

  const updateSession = (threadId: string, updater: (prev: RunSession) => Partial<RunSession>) => {
    if (isThreadDeleted(threadId)) return;
    set((state) => {
      const prev = state.sessions[threadId] ?? freshSession();
      return { sessions: { ...state.sessions, [threadId]: normalizeSession({ ...prev, ...updater(prev) }) } };
    });
  };

  const updateTokenUsage = (threadId: string, reduce: (current: RunTokenUsageByRun) => RunTokenUsageByRun) => {
    if (isThreadDeleted(threadId)) return;
    set(state => {
      const session = state.sessions[threadId] ?? freshSession();
      const tokenUsageByRun = reduce(session.tokenUsageByRun);
      if (tokenUsageByRun === session.tokenUsageByRun) return state;
      return { sessions: { ...state.sessions, [threadId]: { ...session, tokenUsageByRun } } };
    });
  };

  const stopCancelReconciliation = (threadId: string, runId?: string) => {
    const reconciliation = cancelReconciliations.get(threadId);
    if (!reconciliation || (runId && reconciliation.runId !== runId)) return;
    if (reconciliation.timer !== undefined) clearTimeout(reconciliation.timer);
    cancelReconciliations.delete(threadId);
  };

  const acceptStoppedRun = (threadId: string, run: Run) => {
    if (!runStopped(run.status)) return;
    const session = get().sessions[threadId];
    if (!session) return;
    if (session.activeRunId === run.id) {
      session.eventSourceClose?.();
      stopCancelReconciliation(threadId, run.id);
      if (run.status === 'paused') writePersistedRun({ runId: run.id, threadId });
      else removePersistedRun(threadId);
      const context = useContextWindowStore.getState().sessions[threadId];
      if (context) useContextWindowStore.setState(state => ({ sessions: { ...state.sessions, [threadId]: {
        ...context, compacting: false,
        snapshot: context.snapshot ? { ...context.snapshot, status: 'idle' } : null,
      } } }));
    }
    updateSession(threadId, prev => ({
      stoppedRuns: { ...prev.stoppedRuns, [run.id]: { run, sequence: Number(prev.lastEventId ?? 0) } },
      ...(prev.activeRunId === run.id ? { eventSourceClose: null } : {}),
    }));
  };

  const reconcileUnfinishedHistory = (threadId: string) => {
    const session = get().sessions[threadId];
    if (!session) return;
    const runIds = new Set(session.timelineItems.flatMap(item => item.runId
      && ((item.type === 'tool' && ['running', 'interrupted', 'canceled'].includes(item.status))
        || (item.type === 'command' && item.commandSource === 'automatic' && item.status === 'loading')) ? [item.runId] : []));
    for (const runId of runIds) {
      if (session.stoppedRuns?.[runId] || (runId === session.activeRunId && !['done', 'error', 'canceled', 'paused'].includes(session.status))) continue;
      const key = `${threadId}:history:${runId}`;
      if (reconcilingRuns.has(key)) continue;
      reconcilingRuns.add(key);
      const sequence = session.lastEventId;
      void runsApi.get(runId).then(run => {
        const current = get().sessions[threadId];
        if (current && current.lastEventId === sequence && current.status === session.status
          && !isThreadDeleted(threadId) && !isProjectDeleted(run.project_id)) acceptStoppedRun(threadId, run);
      }).catch(() => { /* Unavailable authority does not prove failure. */ })
        .finally(() => reconcilingRuns.delete(key));
    }
  };

  const replayAuthoritativeTerminal = (
    threadId: string,
    runId: string,
    status: 'done' | 'failed' | 'canceled',
    projectId?: string,
  ) => {
    const session = get().sessions[threadId];
    if (!session || session.activeRunId !== runId) {
      stopCancelReconciliation(threadId, runId);
      return;
    }
    stopCancelReconciliation(threadId, runId);
    session.eventSourceClose?.();
    removePersistedRun(threadId);
    patchSession(threadId, {
      stoppedRuns: Object.fromEntries(Object.entries(session.stoppedRuns ?? {}).filter(([id, snapshot]) => id !== runId || snapshot.run.status === status)),
      timelineItems: session.timelineItems.filter(item => !(item.type === 'terminal_notice' && item.runId === runId && item.pauseReason)),
      status: sessionStatusForRun({ status }),
      streamStatus: 'closed',
      progress: null,
      pendingQuestion: null,
      eventSourceClose: null,
    });
    const resolvedProjectId = projectId ?? session.projectId ?? undefined;
    if (resolvedProjectId) void useProjectStore.getState().checkProjectContent(resolvedProjectId);
    void threadsApi.history(threadId)
      .then((history) => {
        const current = get().sessions[threadId];
        if (!current || current.activeRunId !== runId || !['done', 'error', 'canceled', 'paused'].includes(current.status)) return;
        const hydrated = hydrateRunFromHistory(history as unknown as HistoryEntry[]);
        const authoritativeItems = hydrated.items.filter((item) => item.runId === runId);
        updateSession(threadId, (prev) => ({
          timelineItems: ensureTerminalTimelineItem(
            mergeAuthoritativeTimeline(prev.timelineItems, authoritativeItems),
            runId,
            status,
          ),
          ...newerPlan(prev, hydrated.plan, hydrated.session.planSequence),
          tokenUsageByRun: mergeRunTokenUsage(prev.tokenUsageByRun, hydrated.session.tokenUsageByRun),
          lastEventId: hydrated.session.activeRunId === runId ? hydrated.lastEventId : prev.lastEventId,
          nextInputSuggestions: hydrated.session.activeRunId === runId
            ? hydrated.session.nextInputSuggestions
            : prev.nextInputSuggestions,
        }));
      })
      .catch(() => {
        const current = get().sessions[threadId];
        if (!current || current.activeRunId !== runId || !['done', 'error', 'canceled', 'paused'].includes(current.status)) return;
        updateSession(threadId, (prev) => ({
          timelineItems: ensureTerminalTimelineItem(prev.timelineItems, runId, status),
        }));
      });
  };

  const startCancelReconciliation = (threadId: string, runId: string) => {
    stopCancelReconciliation(threadId);
    const reconciliation: CancelReconciliation = { runId };
    cancelReconciliations.set(threadId, reconciliation);

    const schedule = (attempt: number) => {
      reconciliation.timer = setTimeout(async () => {
        if (cancelReconciliations.get(threadId) !== reconciliation) return;
        const before = get().sessions[threadId];
        if (!before || before.activeRunId !== runId || before.status !== 'canceling') {
          stopCancelReconciliation(threadId, runId);
          return;
        }
        try {
          const run = await runsApi.get(runId);
          if (cancelReconciliations.get(threadId) !== reconciliation) return;
          const current = get().sessions[threadId];
          if (!current || current.activeRunId !== runId || current.status !== 'canceling') {
            stopCancelReconciliation(threadId, runId);
            return;
          }
          if (runStopped(run.status)) {
            acceptStoppedRun(threadId, run);
            if (run.status === 'paused') return;
          }
          if (isTerminalRunStatus(run.status)) {
            replayAuthoritativeTerminal(threadId, runId, run.status, run.project_id);
            return;
          }
          patchSession(threadId, { status: 'canceling' });
        } catch {
          // SSE remains authoritative; a failed reconciliation consumes this bounded attempt.
        }
        if (attempt + 1 < CANCEL_RECONCILIATION_DELAYS_MS.length) {
          schedule(attempt + 1);
        } else if (cancelReconciliations.get(threadId) === reconciliation) {
          cancelReconciliations.delete(threadId);
        }
      }, CANCEL_RECONCILIATION_DELAYS_MS[attempt]);
    };

    schedule(0);
  };

  const refreshTarget = (session: RunSession, event: SSEEvent) => {
    if (!session.projectId) return;
    const structuredMutation = event.event === 'tool.completed'
      && (isResourceEditTool(event.data.tool) || event.data.changes !== undefined)
      && event.data.status === 'completed';
    const terminal = event.event === 'run.completed' || event.event === 'run.failed' || event.event === 'run.error' || event.event === 'run.canceled';
    if (structuredMutation || terminal) void useProjectStore.getState().checkProjectContent(session.projectId);
  };

  return {
    sessions: {},
    clearPlan: async (threadId) => {
      const result = await threadsApi.clearPlan(threadId);
      if (!isThreadDeleted(threadId)) updateSession(threadId, prev => newerPlan(prev, result.plan, result.seq));
    },

    createRun: async (threadId, payload, projectId) => {
      if (isThreadDeleted(threadId)) return 'canceled';
      const previousSession = get().sessions[threadId];
      const historyEpoch = currentHistoryEpoch();
      stopCancelReconciliation(threadId);
      payload = { ...payload, client_request_id: payload.client_request_id ?? newClientIdentity('req') };
      get().sessions[threadId]?.eventSourceClose?.();
      const userItem: TimelineItem = {
        id: `user_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`,
        type: 'user_turn',
        text: payload.instruction,
        scope: payload.scope,
        mode: payload.mode,
        skills: payload.skill_ids?.map((id) => ({ id, name: id, description: '' })) ?? [],
        components: payload.component_names?.map((name) => ({
          kind: 'component' as const, id: name, name,
        })) ?? [],
        domSelections: payload.dom_selections?.map(({ selection_id, marker_no, comment, status }) => ({ selection_id, marker_no, comment, status })),
        referenceOrder: payload.reference_order,
        timestamp: Date.now(),
      };
      updateSession(threadId, (prev) => ({
        timelineItems: [...prev.timelineItems, userItem],
        activeRunId: null,
        activeRunModel: null,
        projectId: projectId ?? prev.projectId ?? null,
        status: 'creating',
        runClock: { elapsedMs: 0, runningSince: userItem.timestamp },
        streamStatus: 'idle',
        pendingQuestion: null,
        scope: payload.scope,
        mode: payload.mode,
        progress: null,
        eventSourceClose: null,
        processedEventIds: [],
        lastEventId: undefined,
        originalRequest: payload,
      }));

      try {
        const run = await runsApi.create(threadId, payload);
        if (isThreadDeleted(threadId)) return 'canceled';
        if (run.dropped_mentioned_slide_ids?.length) {
          showGlobalWarning(
            run.dropped_mentioned_slide_ids.length === payload.mentioned_slide_ids?.length
              ? '所点页面均不存在，已按未点名页面继续执行'
              : `已忽略 ${run.dropped_mentioned_slide_ids.length} 个不存在的页面，其余页面继续执行`,
          );
        }
        patchSession(threadId, {
          activeRunId: run.id,
          activeRunModel: run.model_execution?.profile ?? run.model,
          projectId: run.project_id,
          // The HTTP response already confirms acceptance, even before SSE starts.
          nextInputSuggestions: null,
          status: sessionStatusForRun(run),
          originalRequest: {
            ...payload,
            ...(payload.model || !run.model ? {} : { model: run.model }),
          },
        });
        updateSession(threadId, (prev) => ({
          timelineItems: prev.timelineItems.map((item) => (
            item.id === userItem.id && item.type === 'user_turn'
              ? {
                  ...item,
                  runId: run.id,
                  skills: run.skills ?? item.skills,
                  components: run.components ?? item.components,
                }
              : item
          )),
        }));
        if (run.model) useComposerStore.getState().setModelProfileName(run.model);
        writePersistedRun({ runId: run.id, threadId });
        get().subscribeRun(threadId, run.id, run.project_id);
        return 'created';
      } catch (error) {
        if (isThreadDeleted(threadId)) return 'canceled';
        if (error instanceof RequestCanceledError) {
          if (historyEpoch === currentHistoryEpoch()) {
            set((state) => {
              const sessions = { ...state.sessions };
              const live = sessions[threadId];
              if (previousSession) {
                sessions[threadId] = { ...previousSession, ...(live ? newerPlan(previousSession, live.plan, live.planSequence) : {}) };
              } else if (live?.planSequence) {
                sessions[threadId] = freshSession({ plan: live.plan, planSequence: live.planSequence });
              } else delete sessions[threadId];
              return { sessions };
            });
          }
          return 'canceled';
        }
        const detail = errorMessage(error);
        updateSession(threadId, (prev) => ({
          status: 'error',
          streamStatus: 'closed',
          timelineItems: [...prev.timelineItems, {
            id: `create_error_${Date.now()}`,
            type: 'terminal_notice',
            status: 'failed',
            message: `${localizedErrorMessage(detail.message, '运行创建失败')}。请检查后重试，输入内容已保留。`,
            affectedTargets: [],
            technicalMessage: detail.message,
            code: detail.code,
            requestId: detail.requestId,
            retryable: detail.retryable,
            timestamp: Date.now(),
          }],
        }));
        return 'failed';
      }
    },

    subscribeRun: (threadId, runId, projectId) => {
      if (isThreadDeleted(threadId)) return;
      const reconciliation = cancelReconciliations.get(threadId);
      if (reconciliation && reconciliation.runId !== runId) {
        stopCancelReconciliation(threadId);
      }
      get().sessions[threadId]?.eventSourceClose?.();
      patchSession(threadId, {
        activeRunId: runId,
        projectId: projectId ?? get().sessions[threadId]?.projectId ?? null,
        streamStatus: 'connecting',
      });

      let reconcilingStreamFailure = false;
      let close = () => {};
      let closed = false;
      let statusTimer: ReturnType<typeof setTimeout> | undefined;
      const reconcileStatus = async () => {
        if (closed || reconcilingStreamFailure) return;
        reconcilingStreamFailure = true;
        const sequence = get().sessions[threadId]?.lastEventId;
        try {
          const run = await runsApi.get(runId);
          const current = get().sessions[threadId];
          if (closed || !current || current.activeRunId !== runId || current.lastEventId !== sequence) return;
          if (runStopped(run.status)) {
            acceptStoppedRun(threadId, run);
            if (isTerminalRunStatus(run.status)) replayAuthoritativeTerminal(threadId, runId, run.status, run.project_id);
          } else if (current.status !== 'canceling') {
            patchSession(threadId, { status: sessionStatusForRun(run) });
          }
        } catch { /* A disconnected stream alone cannot establish a stopped run. */ }
        finally { reconcilingStreamFailure = false; }
      };
      const scheduleStatus = () => {
        statusTimer = setTimeout(async () => {
          await reconcileStatus();
          if (!closed) scheduleStatus();
        }, 3_000);
      };
      const unsubscribe = subscribeRunEvents(runId, {
        threadId,
        onStatus: (streamStatus) => {
          if (!closed && get().sessions[threadId]?.activeRunId === runId) patchSession(threadId, { streamStatus });
        },
        onUnknown: (eventName) => {
          if (import.meta.env.DEV) console.warn('忽略未知 SSE 事件', eventName);
        },
        onMessage: (event, source) => {
          if (closed || isThreadDeleted(threadId)) return;
          const current = get().sessions[threadId] ?? freshSession();
          if (current.activeRunId !== runId) return;
          if (event.id && current.processedEventIds?.includes(event.id)) return;
          if (event.event === 'run.usage.updated') {
            updateTokenUsage(threadId, usage => reduceRunTokenUsage(usage, event));
            return;
          }
          // A full reconnect replay can include a terminal from before resume.
          // History already restored this prefix; replaying it would close the
          // active subscription before its subsequent run.resumed is delivered.
          if (source === 'replay' && Number(event.id ?? 0) <= Number(current.lastEventId ?? 0)) return;
          if (event.event === 'context.window.updated') {
            useContextWindowStore.getState().update(threadId, event.data);
          }
          receiveCompactionEvent(threadId, event);
          updateSession(threadId, (prev) => {
            const stoppedRuns = { ...prev.stoppedRuns };
            if (event.event === 'run.completed' || event.event === 'run.failed' || event.event === 'run.error' || event.event === 'run.canceled') {
              delete stoppedRuns[runId];
            }
            const nextTimelineItems = reduceSSEEvent(prev.timelineItems, event);
            let status = prev.status === 'creating' ? 'running' : prev.status;
            let pendingQuestion = prev.pendingQuestion;
            const nextInputSuggestions = reduceNextInputSuggestions(prev.nextInputSuggestions, event, prev.activeRunId);

            if (event.event === 'question.asked') {
              status = prev.status === 'canceling' ? 'canceling' : 'waiting';
              pendingQuestion = { id: event.data.question_id };
            } else if (event.event === 'question.answered') {
              status = prev.status === 'canceling' ? 'canceling' : 'running';
              pendingQuestion = null;
            } else if (event.event === 'plan.approval_requested') {
              status = prev.status === 'canceling' ? 'canceling' : 'waiting';
            } else if (event.event === 'plan.approval_answered') {
              pendingQuestion = null;
              if (prev.status !== 'canceling') {
                status = 'running';
              }
            } else if (event.event === 'command.permission_requested') {
              status = prev.status === 'canceling' ? 'canceling' : 'waiting';
              pendingQuestion = null;
            } else if (event.event === 'command.permission_answered') {
              if (prev.status !== 'canceling') {
                status = 'running';
              }
            } else if (event.event === 'scope.expansion_requested') {
              status = prev.status === 'canceling' ? 'canceling' : 'waiting';
              pendingQuestion = null;
            } else if (event.event === 'scope.expansion_answered') {
              if (prev.status !== 'canceling') {
                status = 'running';
              }
            } else if (event.event === 'resource.edit_approval_requested') {
              status = prev.status === 'canceling' ? 'canceling' : 'waiting';
              pendingQuestion = null;
            } else if (event.event === 'resource.edit_approval_answered') {
              if (prev.status !== 'canceling') status = 'running';
            } else if (event.event === 'run.resumed') {
              if (prev.status !== 'canceling') status = 'recovering';
            } else if (event.event === 'run.progress') {
              if (event.data.activity === 'run.canceling') status = 'canceling';
              else if (prev.status !== 'canceling' && prev.status !== 'waiting') status = 'running';
            } else if (event.event === 'run.completed') {
              status = 'done';
              pendingQuestion = null;
            } else if (event.event === 'run.canceled') {
              status = 'canceled';
              pendingQuestion = null;
            } else if (event.event === 'run.failed' || event.event === 'run.error') {
              status = 'error';
              pendingQuestion = null;
            }

            return {
              stoppedRuns,
              runClock: reduceRunClock(prev.runClock, event),
              timelineItems: nextTimelineItems,
              ...(event.event === 'plan.updated' ? newerPlan(prev, reducePlan(prev.plan, event), Number(event.id)) : {}),
              status,
              pendingQuestion,
              progress: reduceRunProgress(prev.progress, status, event),
              activeRunModel: event.event === 'run.progress' && event.data.model_switch
                ? event.data.model_switch.to
                : prev.activeRunModel,
              lastEventId: event.id || prev.lastEventId,
              processedEventIds: event.id
                ? [...(prev.processedEventIds ?? []), event.id].slice(-500)
                : prev.processedEventIds,
              ...(event.event === 'scope.updated' ? { scope: event.data.scope } : {}),
              ...(event.event === 'run.mode_changed' ? { mode: event.data.mode } : {}),
              nextInputSuggestions,
            };
          });

          const updated = get().sessions[threadId] ?? freshSession();
          if (event.event === 'run.resumed' && updated.stoppedRuns?.[runId]
            && Number(event.id ?? 0) > updated.stoppedRuns[runId].sequence) {
            void get().reconcileRun(threadId, runId, event.id, projectId);
          }
          if (event.id && updated.projectId) {
            writePersistedRun({
              runId,
              threadId,
              canceling: updated.status === 'canceling',
            });
          }
          if (event.event === 'run.completed' || event.event === 'run.failed' || event.event === 'run.error' || event.event === 'run.canceled') {
            const contextSession = useContextWindowStore.getState().sessions[threadId];
            if (contextSession?.snapshot) {
              useContextWindowStore.getState().update(threadId, {
                ...contextSession.snapshot,
                status: 'idle',
              });
            }
            void useContextWindowStore.getState().load(threadId, useComposerStore.getState().modelProfileName ?? '');
            stopCancelReconciliation(threadId, runId);
            updated.eventSourceClose?.();
            removePersistedRun(threadId);
            patchSession(threadId, { eventSourceClose: null, streamStatus: 'closed' });
            reconcileUnfinishedHistory(threadId);
          }
          refreshTarget(updated, event);
        },
        onError: () => {
          if (closed) return;
          patchSession(threadId, { streamStatus: 'reconnecting' });
          void reconcileStatus();
        },
      });
      close = () => { closed = true; clearTimeout(statusTimer); unsubscribe(); };
      patchSession(threadId, { eventSourceClose: close });
      scheduleStatus();
    },

    recoverPersistedRuns: async () => {
      const records = Object.values(readPersistedRuns());
      await Promise.all(records.map(async (record) => {
        // Read the snapshot after history so it can settle a cancellation completed during replay.
        const history = await threadsApi.history(record.threadId)
          .then(entries => hydrateRunFromHistory(entries as unknown as HistoryEntry[]))
          .catch(() => undefined);
        const replay = history?.session.activeRunId === record.runId ? history : undefined;
        const canceling = record.canceling || replay?.session.status === 'canceling';
        try {
          const run = await runsApi.get(record.runId);
          if (isThreadDeleted(record.threadId) || isProjectDeleted(run.project_id)) {
            removePersistedRun(record.threadId);
            return;
          }
          const live = get().sessions[record.threadId];
          if (live?.status === 'creating' || (live?.activeRunId && live.activeRunId !== run.id)) return;
          const status = sessionStatusForRun(run, canceling || (live?.activeRunId === run.id && live.status === 'canceling'));
          const hydratedItems = history?.items ?? [];
          const pending = [...hydratedItems].reverse().find((item) =>
            item.runId === run.id && item.type === 'question' && !item.answer);
          updateSession(record.threadId, (prev) => ({
            stoppedRuns: runStopped(run.status)
              ? { ...prev.stoppedRuns, [run.id]: { run, sequence: Number(replay?.lastEventId ?? 0) } }
              : prev.stoppedRuns,
            runClock: replay?.session.runClock,
            tokenUsageByRun: mergeRunTokenUsage(prev.tokenUsageByRun, history?.session.tokenUsageByRun ?? {}),
            activeRunId: run.id,
            activeRunModel: run.model_execution?.profile ?? run.model,
            projectId: run.project_id,
            status,
            streamStatus: status === 'running' || status === 'waiting' || status === 'recovering' || status === 'canceling'
              ? 'connecting'
              : 'closed',
            scope: run.scope,
            mode: run.mode,
            lastEventId: replay?.lastEventId,
            progress: replay?.session.progress ?? null,
            timelineItems: mergeAuthoritativeTimeline(prev.timelineItems, hydratedItems),
            ...(history ? newerPlan(prev, history.plan, history.session.planSequence) : {}),
            nextInputSuggestions: prev.timelineItems.length > 0 ? prev.nextInputSuggestions : replay?.session.nextInputSuggestions ?? null,
            pendingQuestion: status === 'waiting' && pending?.type === 'question'
              ? { id: pending.questionId }
              : null,
            originalRequest: prev.originalRequest ?? requestFromTimeline(hydratedItems, run.id, run.model),
          }));
          if (!runStopped(run.status)) {
            get().subscribeRun(record.threadId, run.id, run.project_id);
            if (status === 'canceling') {
              writePersistedRun({ runId: run.id, threadId: record.threadId, canceling: true });
              startCancelReconciliation(record.threadId, run.id);
            }
          } else if (run.status === 'paused') {
            writePersistedRun({
              runId: run.id,
              threadId: record.threadId,
            });
          } else {
            removePersistedRun(record.threadId);
          }
          reconcileUnfinishedHistory(record.threadId);
        } catch (error) {
          const live = get().sessions[record.threadId];
          if (live?.status === 'creating' || (live?.activeRunId && live.activeRunId !== record.runId)) return;
          if (live?.activeRunId === record.runId && ['paused', 'done', 'error', 'canceled'].includes(live.status)) return;
          if (!(error instanceof APIError) || error.status !== 404) {
            patchSession(record.threadId, {
              activeRunId: record.runId,
              streamStatus: 'reconnecting',
              ...(canceling ? { status: 'canceling' } : {}),
            });
            get().subscribeRun(record.threadId, record.runId);
            return;
          }
          removePersistedRun(record.threadId);
          updateSession(record.threadId, (prev) => ({
            activeRunId: record.runId,
            status: 'error',
            streamStatus: 'closed',
            timelineItems: [...prev.timelineItems, {
              id: `restore_notice_${Date.now()}`,
              type: 'terminal_notice',
              runId: record.runId,
              status: 'failed',
              message: '未找到刷新前的运行记录，已清理。你可以重新发送指令。',
              affectedTargets: [],
              timestamp: Date.now(),
            }],
          }));
        }
      }));
    },

    syncThreadHistory: (threadId, entries, projectId) => {
      const hydrated = hydrateRunFromHistory(entries);
      const runId = hydrated.session.activeRunId;
      const current = get().sessions[threadId] ?? IDLE_SESSION;
      // Consumption is immutable metadata, even when the history's lifecycle
      // snapshot is too old to replace the currently displayed Run.
      updateTokenUsage(threadId, usage => mergeRunTokenUsage(usage, hydrated.session.tokenUsageByRun));
      if ((hydrated.session.planSequence ?? 0) > (current.planSequence ?? 0)) {
        patchSession(threadId, newerPlan(current, hydrated.plan, hydrated.session.planSequence));
      }
      if (current.status === 'creating') return false;
      if (!runId) {
        if (hydrated.items.length > 0 && JSON.stringify(hydrated.items) !== JSON.stringify(current.timelineItems)) {
          updateSession(threadId, (prev) => ({
            timelineItems: mergeAuthoritativeTimeline(prev.timelineItems, hydrated.items),
          }));
        }
        return true;
      }
      const sameRun = current.activeRunId === runId;
      const previousRunEnded = entries.some(entry => entry.run_id === current.activeRunId
        && ['run.completed', 'run.failed', 'run.error', 'run.canceled'].includes(entry.type));
      if (!sameRun && current.activeRunId
        && ['creating', 'running', 'waiting', 'recovering', 'canceling', 'paused'].includes(current.status)
        && !previousRunEnded) return false;
      const serverSequence = Number(hydrated.lastEventId ?? 0);
      const localSequence = Number(current.lastEventId ?? 0);
      if (sameRun && serverSequence < localSequence) return false;
      const newRunEvents = !sameRun || serverSequence > localSequence;
      if (sameRun && serverSequence === localSequence && current.status === hydrated.session.status
        && JSON.stringify(current.timelineItems) === JSON.stringify(hydrated.items)
        && JSON.stringify(current.plan) === JSON.stringify(hydrated.plan)
        && JSON.stringify(current.nextInputSuggestions) === JSON.stringify(hydrated.session.nextInputSuggestions)) return true;
      if (!sameRun) {
        current.eventSourceClose?.();
        stopCancelReconciliation(threadId);
      }
      updateSession(threadId, (prev) => ({
        activeRunId: runId,
        activeRunModel: sameRun ? prev.activeRunModel : null,
        projectId: projectId ?? prev.projectId,
        status: newRunEvents ? withPendingCancellation(hydrated.session.status, sameRun && prev.status === 'canceling') : prev.status,
        scope: newRunEvents ? hydrated.session.scope ?? prev.scope : prev.scope,
        mode: newRunEvents ? hydrated.session.mode ?? prev.mode : prev.mode,
        timelineItems: sameRun ? mergeAuthoritativeTimeline(prev.timelineItems, hydrated.items, newRunEvents ? undefined : runId) : hydrated.items,
        ...newerPlan(prev, hydrated.plan, hydrated.session.planSequence),
        pendingQuestion: newRunEvents ? hydrated.session.pendingQuestion : prev.pendingQuestion,
        runClock: newRunEvents ? hydrated.session.runClock : prev.runClock ?? hydrated.session.runClock,
        nextInputSuggestions: hydrated.session.nextInputSuggestions,
        lastEventId: hydrated.lastEventId,
        streamStatus: sameRun ? prev.streamStatus : 'closed',
        eventSourceClose: sameRun ? prev.eventSourceClose : null,
        progress: newRunEvents ? hydrated.session.progress ?? null : prev.progress,
      }));
      if ((!sameRun && ['running', 'waiting', 'recovering', 'canceling', 'paused'].includes(hydrated.session.status))
        || (sameRun && current.stoppedRuns?.[runId] && serverSequence > current.stoppedRuns[runId].sequence)) {
        void get().reconcileRun(threadId, runId, hydrated.lastEventId, projectId);
      }
      reconcileUnfinishedHistory(threadId);
      return true;
    },

    reconcileRun: async (threadId, runId, lastEventId, projectId) => {
      if (isThreadDeleted(threadId)) return;
      const key = `${threadId}:${runId}`;
      if (reconcilingRuns.has(key)) return;
      reconcilingRuns.add(key);
      const before = get().sessions[threadId];
      try {
        const run = await runsApi.get(runId);
        if (isThreadDeleted(threadId) || isProjectDeleted(run.project_id)) return;
        const current = get().sessions[threadId];
        const currentRunId = current?.activeRunId;
        if (currentRunId && currentRunId !== runId) return;
        if (current?.lastEventId !== before?.lastEventId) return;
        const status = sessionStatusForRun(run, currentRunId === run.id && current?.status === 'canceling');
        patchSession(threadId, {
          ...(!runStopped(run.status) ? {
            stoppedRuns: Object.fromEntries(Object.entries(get().sessions[threadId]?.stoppedRuns ?? {}).filter(([id]) => id !== runId)),
            timelineItems: (get().sessions[threadId]?.timelineItems ?? []).filter(item => !(item.type === 'terminal_notice' && item.runId === runId && item.pauseReason)),
          } : {}),
          activeRunId: run.id,
          activeRunModel: run.model_execution?.profile ?? run.model,
          projectId: run.project_id || projectId || null,
          status,
          streamStatus: run.status === 'paused' || isTerminalRunStatus(run.status) ? 'closed' : 'connecting',
          scope: run.scope,
          mode: run.mode,
          lastEventId: lastEventId ?? get().sessions[threadId]?.lastEventId,
          progress: currentRunId === run.id ? before?.progress ?? null : null,
          pendingQuestion: status === 'waiting' ? get().sessions[threadId]?.pendingQuestion ?? null : null,
        });
        if (runStopped(run.status)) acceptStoppedRun(threadId, run);
        if (isTerminalRunStatus(run.status)) {
          replayAuthoritativeTerminal(threadId, runId, run.status, run.project_id);
          return;
        }
        writePersistedRun({ runId: run.id, threadId, canceling: status === 'canceling' });
        if (run.status !== 'paused') {
          get().subscribeRun(threadId, run.id, run.project_id);
        }
      } catch {
        // Retain history and retry authority through the subscription's status probe.
        const current = get().sessions[threadId];
        if (current?.activeRunId === runId && !current.eventSourceClose
          && !['paused', 'done', 'error', 'canceled'].includes(current.status)) {
          get().subscribeRun(threadId, runId, projectId);
        }
      } finally {
        reconcilingRuns.delete(key);
      }
    },

    answerQuestion: async (threadId, runId, replyTo, content) => {
      try {
        await runsApi.submitInput(runId, { reply_to: replyTo, content });
        const optimisticAnswer = parseQuestionAnswer(content);
        updateSession(threadId, (prev) => {
          if (prev.activeRunId !== runId) return {};
          const timelineItems = optimisticAnswer
            ? prev.timelineItems.map((item) =>
                item.type === 'question' && item.questionId === replyTo && !item.answer
                  ? { ...item, answer: optimisticAnswer }
                  : item)
            : prev.timelineItems;
          if (prev.status === 'canceling') {
            return {
              pendingQuestion: prev.pendingQuestion?.id === replyTo ? null : prev.pendingQuestion,
              timelineItems,
            };
          }
          return {
            status: 'running',
            pendingQuestion: null,
            timelineItems,
          };
        });
        return true;
      } catch (error) {
        const detail = errorMessage(error);
        updateSession(threadId, (prev) => ({
          status: 'waiting',
          timelineItems: [...prev.timelineItems, {
            id: `input_error_${Date.now()}`,
            type: 'terminal_notice',
            status: 'failed',
            message: `${localizedErrorMessage(detail.message, '回答提交失败')}。回答尚未提交，请重试。`,
            affectedTargets: [],
            technicalMessage: detail.message,
            code: detail.code,
            requestId: detail.requestId,
            retryable: detail.retryable,
            timestamp: Date.now(),
          }],
        }));
        return false;
      }
    },

    answerCommandPermission: async (
      threadId,
      runId,
      interactionId,
      callId,
      commandHash,
      decision,
      feedback,
    ) => {
      try {
        await runsApi.submitCommandPermission(runId, {
          interaction_id: interactionId,
          call_id: callId,
          command_hash: commandHash,
          decision,
          feedback,
        });
        updateSession(threadId, (prev) => ({
          status: prev.status === 'waiting' ? 'running' : prev.status,
          timelineItems: prev.timelineItems.map((item) =>
            item.type === 'tool'
              && item.runId === runId
              && item.commandApproval?.interactionId === interactionId
              && item.callId === callId
              && item.commandApproval.commandHash === commandHash
              ? { ...item, commandApproval: { ...item.commandApproval, answer: { decision, feedback } } }
              : item),
        }));
        return true;
      } catch (error) {
        const detail = errorMessage(error);
        showGlobalError(localizedErrorMessage(detail.message, '命令授权提交失败，请重试'));
        try {
          const history = await threadsApi.history(threadId);
          get().syncThreadHistory(threadId, history as unknown as HistoryEntry[]);
        } catch {
          // The existing timeline remains usable when an authoritative refresh is unavailable.
        }
        return false;
      }
    },

    syncResourceApprovalDraft: (record) => {
      if (record.state !== 'pending') return;
      set(state => ({ sessions: Object.fromEntries(Object.entries(state.sessions).map(([threadId, session]) => [threadId, {
        ...session,
        timelineItems: session.timelineItems.map(item => item.type === 'tool' && item.status === 'running' && item.runId === record.run_id
          && item.callId === record.call_id && item.approval?.interactionId === record.interaction_id
          && !item.approval.answer && item.approval.revision <= record.revision
          ? { ...item, target: record.target, approval: { ...item.approval, revision: record.revision, target: record.target } }
          : item),
      }])) }));
    },

    answerScopeExpansion: async (threadId, runId, payload) => {
      try {
        await runsApi.submitScopeExpansion(runId, payload);
        updateSession(threadId, (prev) => ({
          status: prev.status === 'canceling' ? 'canceling' : 'running',
          timelineItems: prev.timelineItems.map((item) => item.type === 'scope_expansion'
            && item.runId === runId
            && item.interactionId === payload.interaction_id
            && item.callId === payload.call_id
            ? { ...item, answer: { ...item.answer, decision: payload.decision, feedback: payload.feedback } }
            : item),
        }));
        return true;
      } catch (error) {
        const detail = errorMessage(error);
        showGlobalError(localizedErrorMessage(detail.message, '范围审批提交失败，请重试'));
        return false;
      }
    },

    cancelRun: async (threadId, runId, reason = 'user_requested') => {
      try {
        const response = await runsApi.cancel(runId, reason);
        if (isTerminalRunStatus(response.status)) {
          void useContextWindowStore.getState().load(threadId, useComposerStore.getState().modelProfileName ?? '');
          if (reason === 'superseded' && response.status === 'canceled') {
            stopCancelReconciliation(threadId, runId);
            removePersistedRun(threadId);
            get().sessions[threadId]?.eventSourceClose?.();
            updateSession(threadId, (prev) => ({
              activeRunId: null,
              activeRunModel: null,
              status: 'canceled',
              streamStatus: 'closed',
              progress: null,
              pendingQuestion: null,
              eventSourceClose: null,
              timelineItems: endPausedRun(prev.timelineItems, runId),
            }));
            return true;
          }
          replayAuthoritativeTerminal(threadId, runId, response.status);
          return true;
        }
        if (reason === 'superseded') return false;
        const session = get().sessions[threadId];
        if (!session || session.activeRunId !== runId) return false;
        patchSession(threadId, { status: 'canceling' });
        if (session.projectId) {
          writePersistedRun({
            runId,
            threadId,
            canceling: true,
          });
        }
        startCancelReconciliation(threadId, runId);
        return true;
      } catch (error) {
        const detail = errorMessage(error);
        updateSession(threadId, (prev) => ({
          timelineItems: [...prev.timelineItems, {
            id: `cancel_error_${Date.now()}`,
            type: 'terminal_notice',
            status: 'failed',
            message: `${localizedErrorMessage(detail.message, '停止运行失败')}。运行仍在继续，请再次停止。`,
            affectedTargets: [],
            technicalMessage: detail.message,
            code: detail.code,
            requestId: detail.requestId,
            retryable: detail.retryable,
            timestamp: Date.now(),
          }],
        }));
        return false;
      }
    },

    steerRun: async (threadId, runId, content, clientMessageId, attachmentIds = [], domSelections = [], referenceOrder = []) => {
      const session = get().sessions[threadId];
      if (session?.activeRunId !== runId || (session.status !== 'running' && session.status !== 'waiting')) return false;
      const itemId = `steering_${clientMessageId}`;
      updateSession(threadId, (prev) => ({
        timelineItems: [...prev.timelineItems, {
          id: itemId,
          type: 'user_turn',
          runId,
          text: content,
		  domSelections: domSelections.map(({ selection_id, marker_no, comment, status }) => ({ selection_id, marker_no, comment, status })),
		  referenceOrder,
          deliveryStatus: 'sending',
          clientMessageId,
          timestamp: Date.now(),
        }],
      }));
      try {
        await runsApi.steer(runId, {
          expected_run_id: runId,
          client_message_id: clientMessageId,
          content,
			...(attachmentIds.length > 0 ? { attachment_ids: attachmentIds } : {}),
			...(domSelections.length > 0 ? { dom_selections: domSelections } : {}),
			...(referenceOrder.length > 0 ? { reference_order: referenceOrder } : {}),
        });
        updateSession(threadId, (prev) => ({
          timelineItems: prev.timelineItems.map((item) =>
            item.id === itemId && item.type === 'user_turn'
              ? { ...item, deliveryStatus: 'accepted' }
              : item),
          nextInputSuggestions: null,
        }));
        return true;
      } catch (error) {
        const detail = errorMessage(error);
        updateSession(threadId, (prev) => ({
          timelineItems: prev.timelineItems.map((item) =>
            item.id === itemId && item.type === 'user_turn'
              ? {
                ...item,
                deliveryStatus: 'rejected',
                rejectionCode: detail.code,
              }
              : item),
        }));
        return false;
      }
    },

    closeSessions: (threadIds) => {
      const { sessions } = get();
      threadIds.forEach((id) => {
        stopCancelReconciliation(id);
        sessions[id]?.eventSourceClose?.();
      });
      set((state) => {
        const next = { ...state.sessions };
        threadIds.forEach((id) => {
          if (next[id]) next[id] = { ...next[id], eventSourceClose: null, streamStatus: 'closed' };
        });
        return { sessions: next };
      });
    },

    dropSessions: (threadIds) => {
      const { sessions } = get();
      threadIds.forEach((id) => {
        stopCancelReconciliation(id);
        sessions[id]?.eventSourceClose?.();
        removePersistedRun(id);
      });
      set((state) => {
        const next = { ...state.sessions };
        threadIds.forEach((id) => { delete next[id]; });
        return { sessions: next };
      });
    },

    hydrateTimeline: (threadId, items, plan, session, lastEventId, run) => {
      if (isThreadDeleted(threadId)) return;
      const current = get().sessions[threadId];
      if (current?.timelineItems.length) return;
      const sameRun = current?.activeRunId === session?.activeRunId;
      if (!sameRun) {
        stopCancelReconciliation(threadId);
      }
      const canceling = (sameRun && current?.status === 'canceling') ||
        Boolean(run?.cancel_requested_at && run.id === session?.activeRunId);
      patchSession(threadId, {
        ...(run && runStopped(run.status) ? { stoppedRuns: { [run.id]: { run, sequence: Number(lastEventId ?? 0) } } } : {}),
        timelineItems: items,
        ...newerPlan(current ?? IDLE_SESSION, plan ?? null, session?.planSequence),
        activeRunId: session?.activeRunId ?? null,
        activeRunModel: null,
        status: withPendingCancellation(session?.status ?? 'idle', canceling),
        progress: session?.progress ?? null,
        scope: session?.scope ?? IDLE_SESSION.scope,
        mode: session?.mode ?? IDLE_SESSION.mode,
        pendingQuestion: session?.pendingQuestion ?? null,
        runClock: session?.runClock,
        tokenUsageByRun: mergeRunTokenUsage(get().sessions[threadId]?.tokenUsageByRun ?? {}, session?.tokenUsageByRun ?? {}),
        nextInputSuggestions: session?.nextInputSuggestions ?? null,
        lastEventId,
        originalRequest: requestFromTimeline(items, session?.activeRunId),
      });
      reconcileUnfinishedHistory(threadId);
    },
  };
});
