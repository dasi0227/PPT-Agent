import { create } from 'zustand';
import { APIError } from '../api/client';
import { runsApi } from '../api/runs';
import { subscribeRunEvents } from '../api/sse';
import {
  CreateRunRequest,
  ExecutionStrategy,
  PlanState,
  Run,
  RunInteraction,
  RunTarget,
  SSEEvent,
} from '../api/types';
import { TimelineItem, reducePlan, reduceSSEEvent } from '../features/agent/eventReducer';
import { useBlueprintStore } from './blueprintStore';
import { useProjectStore } from './projectStore';

export type { PlanState } from '../api/types';

export type RunStatus = 'idle' | 'creating' | 'running' | 'done' | 'error' | 'canceled' | 'needs_input';
export type StreamStatus = 'idle' | 'connecting' | 'open' | 'reconnecting' | 'closed';

export interface RunSession {
  activeRunId: string | null;
  projectId?: string | null;
  status: RunStatus;
  streamStatus?: StreamStatus;
  target: RunTarget;
  interaction: RunInteraction;
  timelineItems: TimelineItem[];
  pendingInput: { id: string; prompt: string; choices?: string[] } | null;
  progress: { stage: string; current?: number; total?: number } | null;
  eventSourceClose: (() => void) | null;
  plan: PlanState | null;
  strategy?: ExecutionStrategy | null;
  lastEventId?: string;
  processedEventIds?: string[];
}

export const IDLE_SESSION: RunSession = Object.freeze<RunSession>({
  activeRunId: null,
  projectId: null,
  status: 'idle',
  streamStatus: 'idle',
  target: { artifact: 'presentation', level: 'slide' },
  interaction: { intent: 'apply', clarification: 'when_blocked' },
  timelineItems: [],
  pendingInput: null,
  progress: null,
  eventSourceClose: null,
  plan: null,
  strategy: null,
  processedEventIds: [],
});

interface PersistedActiveRun {
  runId: string;
  threadId: string;
  projectId: string;
  lastEventId?: string;
}

const ACTIVE_RUNS_KEY = 'ppt-agent-active-runs-v1';

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
    retryable: true,
  };
}

function localizedErrorMessage(message: string, fallback: string): string {
  return /[\u3400-\u9fff]/u.test(message) ? message : fallback;
}

function terminalStatus(status: Run['status']): RunStatus {
  if (status === 'done') return 'done';
  if (status === 'failed') return 'error';
  if (status === 'canceled') return 'canceled';
  if (status === 'waiting') return 'needs_input';
  return 'running';
}

interface RunStoreV2 {
  sessions: Record<string, RunSession>;
  getSession: (threadId: string) => RunSession;
  createRun: (threadId: string, payload: CreateRunRequest, projectId?: string) => Promise<boolean>;
  subscribeRun: (threadId: string, runId: string, lastEventId?: string, projectId?: string) => void;
  recoverPersistedRuns: () => Promise<void>;
  replyNeedsInput: (threadId: string, runId: string, replyTo: string, content: string) => Promise<boolean>;
  cancelRun: (threadId: string, runId: string) => Promise<void>;
  clearRun: (threadId: string) => void;
  closeSessions: (threadIds: string[]) => void;
  rekeySession: (oldId: string, newId: string) => void;
  dropSessions: (threadIds: string[]) => void;
  hydrateTimeline: (threadId: string, items: TimelineItem[], plan?: PlanState | null, strategy?: ExecutionStrategy | null) => void;
}

export const useRunStore = create<RunStoreV2>((set, get) => {
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

  const refreshTarget = (session: RunSession, event: SSEEvent) => {
    if (event.event !== 'run.completed' || !session.projectId) return;
    const target = event.data.outcome?.target ?? session.target;
    if (target.level === 'slide' && target.slide_id) {
      if (target.artifact === 'presentation') {
        void useProjectStore.getState().loadProjectSlides(session.projectId);
      }
      void useBlueprintStore.getState().refreshSlide(session.projectId, target.slide_id);
      return;
    }
    if (target.artifact === 'presentation') {
      void useProjectStore.getState().loadProjectSlides(session.projectId);
    }
    void useBlueprintStore.getState().loadProject(session.projectId);
  };

  return {
    sessions: {},

    getSession: (threadId) => get().sessions[threadId] ?? IDLE_SESSION,

    createRun: async (threadId, payload, projectId) => {
      get().sessions[threadId]?.eventSourceClose?.();
      const userItem: TimelineItem = {
        id: `user_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`,
        type: 'user_turn',
        text: payload.instruction,
        target: payload.target,
        interaction: payload.interaction,
        timestamp: Date.now(),
      };
      updateSession(threadId, (prev) => ({
        timelineItems: [...prev.timelineItems, userItem],
        activeRunId: null,
        projectId: projectId ?? prev.projectId ?? null,
        status: 'creating',
        streamStatus: 'idle',
        pendingInput: null,
        target: payload.target,
        interaction: payload.interaction,
        progress: null,
        plan: null,
        strategy: null,
        eventSourceClose: null,
        processedEventIds: [],
        lastEventId: undefined,
      }));

      try {
        const run = await runsApi.create(threadId, payload);
        patchSession(threadId, {
          activeRunId: run.id,
          projectId: run.project_id,
          status: run.status === 'waiting' ? 'needs_input' : 'running',
        });
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
            type: 'error',
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
            let pendingInput = prev.pendingInput;
            let strategy = prev.strategy;
            let progress = prev.progress;
            const nextPlan = reducePlan(prev.plan, event);

            if (event.event === 'needs_input') {
              status = 'needs_input';
              pendingInput = { id: event.data.id, prompt: event.data.prompt, choices: event.data.choices };
            } else if (event.event === 'strategy.selected') {
              strategy = event.data.strategy;
            } else if (event.event === 'run.completed') {
              status = 'done';
              pendingInput = null;
              progress = null;
            } else if (event.event === 'run.failed') {
              status = 'error';
              pendingInput = null;
              progress = null;
            } else if (event.event === 'run.canceled') {
              status = 'canceled';
              pendingInput = null;
              progress = null;
            } else if (event.event === 'stage.started') {
              progress = { stage: String(event.data.stage ?? ''), current: 0, total: 0 };
            }

            if (nextPlan && progress) {
              progress = {
                ...progress,
                current: nextPlan.steps.filter((step) => step.status === 'completed').length,
                total: nextPlan.steps.length,
              };
            }

            return {
              timelineItems: reduceSSEEvent(prev.timelineItems, event),
              plan: nextPlan,
              status,
              pendingInput,
              progress,
              strategy,
              lastEventId: event.id || prev.lastEventId,
              processedEventIds: event.id
                ? [...(prev.processedEventIds ?? []), event.id].slice(-500)
                : prev.processedEventIds,
            };
          });

          const updated = get().sessions[threadId] ?? freshSession();
          if (event.id && updated.projectId) {
            writePersistedRun({ runId, threadId, projectId: updated.projectId, lastEventId: event.id });
          }
          if (event.event === 'run.completed' || event.event === 'run.failed' || event.event === 'run.canceled') {
            updated.eventSourceClose?.();
            removePersistedRun(threadId);
            patchSession(threadId, { eventSourceClose: null, streamStatus: 'closed' });
            refreshTarget(updated, event);
          }
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
          const status = terminalStatus(run.status);
          patchSession(record.threadId, {
            activeRunId: run.id,
            projectId: run.project_id,
            status,
            streamStatus: status === 'running' || status === 'needs_input' ? 'connecting' : 'closed',
            target: run.target,
            interaction: run.interaction,
            lastEventId: record.lastEventId,
          });
          if (run.status === 'pending' || run.status === 'running' || run.status === 'waiting') {
            get().subscribeRun(record.threadId, run.id, record.lastEventId, run.project_id);
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
              type: 'error',
              message: '未找到刷新前的运行记录，已清理。你可以重新发送指令。',
              retryable: false,
              timestamp: Date.now(),
            }],
          }));
        }
      }));
    },

    replyNeedsInput: async (threadId, runId, replyTo, content) => {
      try {
        await runsApi.submitInput(runId, { reply_to: replyTo, content });
        patchSession(threadId, { status: 'running', pendingInput: null });
        return true;
      } catch (error) {
        const detail = errorMessage(error);
        updateSession(threadId, (prev) => ({
          status: 'needs_input',
          timelineItems: [...prev.timelineItems, {
            id: `input_error_${Date.now()}`,
            type: 'error',
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
        await runsApi.cancel(runId);
        get().sessions[threadId]?.eventSourceClose?.();
        removePersistedRun(threadId);
        patchSession(threadId, {
          status: 'canceled',
          streamStatus: 'closed',
          pendingInput: null,
          eventSourceClose: null,
        });
      } catch (error) {
        const detail = errorMessage(error);
        updateSession(threadId, (prev) => ({
          timelineItems: [...prev.timelineItems, {
            id: `cancel_error_${Date.now()}`,
            type: 'error',
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

    clearRun: (threadId) => {
      get().sessions[threadId]?.eventSourceClose?.();
      removePersistedRun(threadId);
      patchSession(threadId, freshSession());
    },

    closeSessions: (threadIds) => {
      const { sessions } = get();
      threadIds.forEach((id) => sessions[id]?.eventSourceClose?.());
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
        sessions[id]?.eventSourceClose?.();
        removePersistedRun(id);
      });
      set((state) => {
        const next = { ...state.sessions };
        threadIds.forEach((id) => { delete next[id]; });
        return { sessions: next };
      });
    },

    hydrateTimeline: (threadId, items, plan, strategy) => {
      const existing = get().sessions[threadId]?.timelineItems ?? [];
      if (existing.length > 0) return;
      patchSession(threadId, {
        timelineItems: items,
        plan: plan ?? null,
        strategy: strategy ?? null,
      });
    },
  };
});
