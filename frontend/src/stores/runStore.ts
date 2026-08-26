import { create } from 'zustand';
import { APIError } from '../api/client';
import { runsApi } from '../api/runs';
import { subscribeRunEvents } from '../api/sse';
import { threadsApi } from '../api/threads';
import {
  CreateRunRequest,
  PlanState,
  PublicTarget,
  Run,
  RunMode,
  RunProgressStage,
  RunScope,
  SSEEvent,
} from '../api/types';
import { TimelineItem, reducePlan, reduceSSEEvent } from '../features/agent/eventReducer';
import {
  hydrateRunFromHistory,
  type HistoryEntry,
  type HistorySessionState,
} from '../features/agent/historyHydrator';
import { useProjectStore } from './projectStore';
import { useComposerStore } from './composerStore';
import { newClientIdentity } from '../lib/clientIdentity';

export type { PlanState } from '../api/types';

export type RunStatus = 'idle' | 'creating' | 'running' | 'waiting' | 'canceling' | 'done' | 'error' | 'canceled';
export type StreamStatus = 'idle' | 'connecting' | 'open' | 'reconnecting' | 'closed';

export interface RunSession {
  activeRunId: string | null;
  projectId?: string | null;
  status: RunStatus;
  streamStatus?: StreamStatus;
  scope: RunScope;
  mode: RunMode;
  timelineItems: TimelineItem[];
  pendingQuestion: { id: string; prompt: string } | null;
  progress: {
    stage: RunProgressStage;
    text: string;
    target?: PublicTarget;
    current?: number;
    total?: number;
    unit?: string;
  } | null;
  eventSourceClose: (() => void) | null;
  plan: PlanState | null;
  lastEventId?: string;
  processedEventIds?: string[];
  originalRequest?: CreateRunRequest;
}

export const IDLE_SESSION: RunSession = Object.freeze<RunSession>({
  activeRunId: null,
  projectId: null,
  status: 'idle',
  streamStatus: 'idle',
  scope: { artifact: 'ppt', level: 'slide' },
  mode: 'execute',
  timelineItems: [],
  pendingQuestion: null,
  progress: null,
  eventSourceClose: null,
  plan: null,
  processedEventIds: [],
});

interface PersistedActiveRun {
  runId: string;
  threadId: string;
  projectId: string;
  lastEventId?: string;
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

function terminalStatus(status: Run['status']): RunStatus {
  if (status === 'done') return 'done';
  if (status === 'failed') return 'error';
  if (status === 'canceled') return 'canceled';
  if (status === 'waiting') return 'waiting';
  return 'running';
}

function isTerminalRunStatus(status: string): status is 'done' | 'failed' | 'canceled' {
  return status === 'done' || status === 'failed' || status === 'canceled';
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
  return {
    client_request_id: newClientIdentity('req'),
    ...(model ? { model } : {}),
    scope: original.scope as RunScope,
    mode: original.mode as RunMode,
    instruction: original.text,
  };
}

interface RunStoreV2 {
  sessions: Record<string, RunSession>;
  getSession: (threadId: string) => RunSession;
  createRun: (threadId: string, payload: CreateRunRequest, projectId?: string) => Promise<boolean>;
  subscribeRun: (threadId: string, runId: string, lastEventId?: string, projectId?: string) => void;
  recoverPersistedRuns: () => Promise<void>;
  answerQuestion: (threadId: string, runId: string, replyTo: string, content: string) => Promise<boolean>;
  cancelRun: (threadId: string, runId: string) => Promise<void>;
  steerRun: (threadId: string, runId: string, content: string, clientMessageId: string) => Promise<boolean>;
  retryRun: (threadId: string) => Promise<boolean>;
  clearRun: (threadId: string) => void;
  closeSessions: (threadIds: string[]) => void;
  rekeySession: (oldId: string, newId: string) => void;
  dropSessions: (threadIds: string[]) => void;
  hydrateTimeline: (
    threadId: string,
    items: TimelineItem[],
    plan?: PlanState | null,
    session?: HistorySessionState,
    lastEventId?: string,
  ) => void;
}

export const useRunStore = create<RunStoreV2>((set, get) => {
  const cancelReconciliations = new Map<string, CancelReconciliation>();

  const patchSession = (threadId: string, patch: Partial<RunSession>) => {
    set((state) => {
      const prev = state.sessions[threadId] ?? freshSession();
      return { sessions: { ...state.sessions, [threadId]: { ...prev, ...patch } } };
    });
  };

  const updateSession = (threadId: string, updater: (prev: RunSession) => Partial<RunSession>) => {
    set((state) => {
      const prev = state.sessions[threadId] ?? freshSession();
      return { sessions: { ...state.sessions, [threadId]: { ...prev, ...updater(prev) } } };
    });
  };

  const stopCancelReconciliation = (threadId: string, runId?: string) => {
    const reconciliation = cancelReconciliations.get(threadId);
    if (!reconciliation || (runId && reconciliation.runId !== runId)) return;
    if (reconciliation.timer !== undefined) clearTimeout(reconciliation.timer);
    cancelReconciliations.delete(threadId);
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
    patchSession(threadId, { status: terminalStatus(status), progress: null, pendingQuestion: null });
    get().subscribeRun(
      threadId,
      runId,
      session.lastEventId,
      projectId ?? session.projectId ?? undefined,
    );
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
    const structuredMutation = event.event === 'tool.completed' && event.data.tool === 'mutate_ppt' && event.data.status === 'completed';
    const terminal = event.event === 'run.finished';
    if (structuredMutation || terminal) void useProjectStore.getState().loadProjectContent(session.projectId);
  };

  return {
    sessions: {},

    getSession: (threadId) => get().sessions[threadId] ?? IDLE_SESSION,

    createRun: async (threadId, payload, projectId) => {
      stopCancelReconciliation(threadId);
      payload = { ...payload, client_request_id: payload.client_request_id ?? newClientIdentity('req') };
      get().sessions[threadId]?.eventSourceClose?.();
      const userItem: TimelineItem = {
        id: `user_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`,
        type: 'user_turn',
        text: payload.instruction,
        scope: payload.scope,
        mode: payload.mode,
        timestamp: Date.now(),
      };
      updateSession(threadId, (prev) => ({
        timelineItems: [...prev.timelineItems, userItem],
        activeRunId: null,
        projectId: projectId ?? prev.projectId ?? null,
        status: 'creating',
        streamStatus: 'idle',
        pendingQuestion: null,
        scope: payload.scope,
        mode: payload.mode,
        progress: null,
        plan: null,
        eventSourceClose: null,
        processedEventIds: [],
        lastEventId: undefined,
        originalRequest: payload,
      }));

      try {
        const run = await runsApi.create(threadId, payload);
        patchSession(threadId, {
          activeRunId: run.id,
          projectId: run.project_id,
          status: run.status === 'waiting' ? 'waiting' : 'running',
          originalRequest: {
            ...payload,
            ...(payload.model || !run.model ? {} : { model: run.model }),
          },
        });
        if (run.model) useComposerStore.getState().setModelProfileName(run.model);
        writePersistedRun({ runId: run.id, threadId, projectId: run.project_id });
        get().subscribeRun(threadId, run.id, undefined, run.project_id);
        return true;
      } catch (error) {
        const detail = errorMessage(error);
        updateSession(threadId, (prev) => ({
          status: 'error',
          streamStatus: 'closed',
          timelineItems: [...prev.timelineItems, {
            id: `create_error_${Date.now()}`,
            type: 'terminal_notice',
            status: 'failed',
            message: `${localizedErrorMessage(detail.message, '运行创建失败')}。请检查后重试，输入内容已保留。`,
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

    subscribeRun: (threadId, runId, lastEventId, projectId) => {
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

      const close = subscribeRunEvents(runId, {
        lastEventId,
        onStatus: (streamStatus) => patchSession(threadId, { streamStatus }),
        onUnknown: (eventName) => {
          if (import.meta.env.DEV) console.warn('忽略未知 SSE 事件', eventName);
        },
        onMessage: (event) => {
          const current = get().sessions[threadId] ?? freshSession();
          if (event.id && current.processedEventIds?.includes(event.id)) return;
          updateSession(threadId, (prev) => {
            let status = prev.status === 'creating' ? 'running' : prev.status;
            let pendingQuestion = prev.pendingQuestion;
            let progress = prev.progress;
            const nextPlan = reducePlan(prev.plan, event);

            if (event.event === 'question.asked') {
              status = prev.status === 'canceling' ? 'canceling' : 'waiting';
              pendingQuestion = { id: event.data.question_id, prompt: event.data.questions?.[0]?.title ?? event.data.prompt };
              progress = null;
            } else if (event.event === 'question.answered') {
              status = prev.status === 'canceling' ? 'canceling' : 'running';
              pendingQuestion = null;
            } else if (event.event === 'plan.approval_requested') {
              status = prev.status === 'canceling' ? 'canceling' : 'waiting';
              progress = null;
            } else if (event.event === 'plan.approval_answered') {
              pendingQuestion = null;
              if (prev.status !== 'canceling') {
                if (event.data.decision === 'cancel') {
                  status = 'canceling';
                  progress = { stage: 'thinking', text: '正在停止任务' };
                } else {
                  status = 'running';
                  progress = {
                    stage: 'thinking',
                    text: event.data.decision === 'approve' ? '正在启动执行' : '正在调整计划',
                  };
                }
              }
            } else if (event.event === 'run.progress') {
              progress = {
                stage: event.data.stage,
                text: event.data.text,
                target: event.data.target,
                current: event.data.progress?.current,
                total: event.data.progress?.total,
                unit: event.data.progress?.unit,
              };
            } else if (event.event === 'run.finished') {
              status = event.data.status === 'completed'
                ? 'done'
                : event.data.status === 'canceled'
                  ? 'canceled'
                  : 'error';
              pendingQuestion = null;
              progress = null;
            }

            return {
              timelineItems: reduceSSEEvent(prev.timelineItems, event),
              plan: nextPlan,
              status,
              pendingQuestion,
              progress,
              lastEventId: event.id || prev.lastEventId,
              processedEventIds: event.id
                ? [...(prev.processedEventIds ?? []), event.id].slice(-500)
                : prev.processedEventIds,
            };
          });

          const updated = get().sessions[threadId] ?? freshSession();
          if (event.id && updated.projectId) {
            writePersistedRun({
              runId,
              threadId,
              projectId: updated.projectId,
              lastEventId: event.id,
              canceling: updated.status === 'canceling',
            });
          }
          if (event.event === 'run.finished') {
            stopCancelReconciliation(threadId, runId);
            updated.eventSourceClose?.();
            removePersistedRun(threadId);
            patchSession(threadId, { eventSourceClose: null, streamStatus: 'closed' });
          }
          refreshTarget(updated, event);
        },
        onError: () => {
          patchSession(threadId, { streamStatus: 'reconnecting' });
        },
      });

      patchSession(threadId, { eventSourceClose: close });
    },

    recoverPersistedRuns: async () => {
      const records = Object.values(readPersistedRuns());
      await Promise.all(records.map(async (record) => {
        try {
          const run = await runsApi.get(record.runId);
          const status = record.canceling && !isTerminalRunStatus(run.status)
            ? 'canceling'
            : terminalStatus(run.status);
          let hydratedItems: TimelineItem[] = [];
          let hydratedPlan: PlanState | null = null;
          try {
            const history = await threadsApi.history(record.threadId);
            const hydrated = hydrateRunFromHistory(history as unknown as HistoryEntry[]);
            hydratedItems = hydrated.items;
            hydratedPlan = hydrated.plan;
          } catch {
            // SSE replay still recovers the active suffix when history is temporarily unavailable.
          }
          const pending = [...hydratedItems].reverse().find((item) =>
            item.type === 'question' && !item.answer);
          updateSession(record.threadId, (prev) => ({
            activeRunId: run.id,
            projectId: run.project_id,
            status,
            streamStatus: status === 'running' || status === 'waiting' || status === 'canceling'
              ? 'connecting'
              : 'closed',
            scope: run.scope,
            mode: run.mode,
            lastEventId: record.lastEventId,
            timelineItems: prev.timelineItems.length > 0 ? prev.timelineItems : hydratedItems,
            plan: prev.plan ?? hydratedPlan,
            pendingQuestion: status === 'waiting' && pending?.type === 'question'
              ? { id: pending.questionId, prompt: pending.prompt }
              : null,
            originalRequest: prev.originalRequest ?? requestFromTimeline(hydratedItems, run.id, run.model),
          }));
          if (run.status === 'pending' || run.status === 'running' || run.status === 'waiting') {
            get().subscribeRun(record.threadId, run.id, record.lastEventId, run.project_id);
            if (status === 'canceling') {
              startCancelReconciliation(record.threadId, run.id);
            }
          } else {
            removePersistedRun(record.threadId);
          }
        } catch {
          removePersistedRun(record.threadId);
          updateSession(record.threadId, (prev) => ({
            status: 'idle',
            streamStatus: 'closed',
            timelineItems: [...prev.timelineItems, {
              id: `restore_notice_${Date.now()}`,
              type: 'terminal_notice',
              status: 'failed',
              message: '未找到刷新前的运行记录，已清理。你可以重新发送指令。',
              timestamp: Date.now(),
            }],
          }));
        }
      }));
    },

    answerQuestion: async (threadId, runId, replyTo, content) => {
      try {
        await runsApi.submitInput(runId, { reply_to: replyTo, content });
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

    cancelRun: async (threadId, runId) => {
      try {
        const response = await runsApi.cancel(runId);
        if (isTerminalRunStatus(response.status)) {
          replayAuthoritativeTerminal(threadId, runId, response.status);
          return;
        }
        const session = get().sessions[threadId];
        if (!session || session.activeRunId !== runId) return;
        patchSession(threadId, { status: 'canceling' });
        if (session.projectId) {
          writePersistedRun({
            runId,
            threadId,
            projectId: session.projectId,
            lastEventId: session.lastEventId,
            canceling: true,
          });
        }
        startCancelReconciliation(threadId, runId);
      } catch (error) {
        const detail = errorMessage(error);
        updateSession(threadId, (prev) => ({
          timelineItems: [...prev.timelineItems, {
            id: `cancel_error_${Date.now()}`,
            type: 'terminal_notice',
            status: 'failed',
            message: `${localizedErrorMessage(detail.message, '停止运行失败')}。运行仍在继续，请再次停止。`,
            technicalMessage: detail.message,
            code: detail.code,
            requestId: detail.requestId,
            retryable: detail.retryable,
            timestamp: Date.now(),
          }],
        }));
      }
    },

    steerRun: async (threadId, runId, content, clientMessageId) => {
      const itemId = `steering_${clientMessageId}`;
      updateSession(threadId, (prev) => ({
        timelineItems: [...prev.timelineItems, {
          id: itemId,
          type: 'user_turn',
          runId,
          text: content,
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
        });
        updateSession(threadId, (prev) => ({
          timelineItems: prev.timelineItems.map((item) =>
            item.id === itemId && item.type === 'user_turn'
              ? { ...item, deliveryStatus: 'accepted' }
              : item),
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

    retryRun: async (threadId) => {
      const session = get().sessions[threadId];
      if (!session?.originalRequest || !session.projectId) return false;
      const selectedModel = useComposerStore.getState().modelProfileName;
      return get().createRun(threadId, {
        ...session.originalRequest,
        ...(selectedModel ? { model: selectedModel } : {}),
        client_request_id: newClientIdentity('req'),
      }, session.projectId);
    },

    clearRun: (threadId) => {
      stopCancelReconciliation(threadId);
      get().sessions[threadId]?.eventSourceClose?.();
      removePersistedRun(threadId);
      patchSession(threadId, freshSession());
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

    rekeySession: (oldId, newId) => {
      if (oldId === newId) return;
      stopCancelReconciliation(oldId);
      stopCancelReconciliation(newId);
      set((state) => {
        const existing = state.sessions[oldId];
        if (!existing) return {};
        const next = { ...state.sessions };
        delete next[oldId];
        next[newId] = existing;
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

    hydrateTimeline: (threadId, items, plan, session, lastEventId) => {
      const existing = get().sessions[threadId]?.timelineItems ?? [];
      if (existing.length > 0) return;
      if (session?.activeRunId !== get().sessions[threadId]?.activeRunId) {
        stopCancelReconciliation(threadId);
      }
      patchSession(threadId, {
        timelineItems: items,
        plan: plan ?? null,
        activeRunId: session?.activeRunId ?? null,
        status: session?.status ?? 'idle',
        scope: session?.scope ?? IDLE_SESSION.scope,
        mode: session?.mode ?? IDLE_SESSION.mode,
        pendingQuestion: session?.pendingQuestion ?? null,
        lastEventId,
        originalRequest: requestFromTimeline(items, session?.activeRunId),
      });
    },
  };
});
