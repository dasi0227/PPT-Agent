import { create } from 'zustand';
import { runsApi } from '../api/runs';
import { subscribeRunEvents } from '../api/sse';
import { RunPayload, RunScope, PlanStep } from '../api/types';
import { TimelineItem, reduceSSEEvent } from '../features/agent/eventReducer';

export interface PlanState {
  id: string;
  title: string;
  steps: PlanStep[];
}

export type RunStatus = 'idle' | 'running' | 'done' | 'error' | 'needs_input';

export interface RunSession {
  activeRunId: string | null;
  status: RunStatus;
  mode: string;
  scope: RunScope;
  timelineItems: TimelineItem[];
  pendingInput: { id: string; prompt: string; choices?: string[] } | null;
  progress: { stage: string; current: number; total: number } | null;
  eventSourceClose: (() => void) | null;
  plan: PlanState | null;   // v2 M3 填充
}

// 稳定的 idle 空会话常量：getSession 对缺失 key 返回它，避免组件读到 undefined。
export const IDLE_SESSION: RunSession = Object.freeze({
  activeRunId: null,
  status: 'idle',
  mode: 'normal',
  scope: 'current',
  timelineItems: [],
  pendingInput: null,
  progress: null,
  eventSourceClose: null,
  plan: null,
});

function freshSession(overrides: Partial<RunSession> = {}): RunSession {
  return {
    activeRunId: null,
    status: 'idle',
    mode: 'normal',
    scope: 'current',
    timelineItems: [],
    pendingInput: null,
    progress: null,
    eventSourceClose: null,
    plan: null,
    ...overrides,
  };
}

interface RunStoreV2 {
  sessions: Record<string, RunSession>;   // key = threadId

  getSession: (threadId: string) => RunSession;
  createRun: (threadId: string, payload: RunPayload) => Promise<void>;
  subscribeRun: (threadId: string, runId: string, lastEventId?: string) => void;
  replyNeedsInput: (threadId: string, runId: string, replyTo: string, content: string) => Promise<void>;
  cancelRun: (threadId: string, runId: string) => Promise<void>;
  clearRun: (threadId: string) => void;
  closeSessions: (threadIds: string[]) => void;
}

export const useRunStore = create<RunStoreV2>((set, get) => {
  // 只更新指定 threadId 的分片，绝不串写别的 thread。
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

  return {
    sessions: {},

    getSession: (threadId) => get().sessions[threadId] ?? IDLE_SESSION,

    createRun: async (threadId, payload) => {
      try {
        // 关闭该 thread 之前的连接（仅本分片）
        get().sessions[threadId]?.eventSourceClose?.();

        patchSession(threadId, {
          activeRunId: null,
          status: 'running',
          timelineItems: [],
          pendingInput: null,
          mode: payload.mode ?? 'normal',
          scope: payload.scope ?? 'current',
          progress: null,
          plan: null,
          eventSourceClose: null,
        });

        const run = await runsApi.create(threadId, payload);
        patchSession(threadId, { activeRunId: run.id });
        get().subscribeRun(threadId, run.id);
      } catch (err) {
        patchSession(threadId, { status: 'error' });
        console.error(err);
      }
    },

    subscribeRun: (threadId, runId, lastEventId) => {
      get().sessions[threadId]?.eventSourceClose?.();

      const close = subscribeRunEvents(runId, {
        lastEventId,
        onMessage: (event) => {
          updateSession(threadId, (prev) => {
            let status = prev.status;
            let pendingInput = prev.pendingInput;
            let progress = prev.progress;

            if (event.event === 'needs_input') {
              status = 'needs_input';
              pendingInput = { id: event.data.id, prompt: event.data.prompt, choices: event.data.choices };
            } else if (event.event === 'done') {
              status = 'done';
              pendingInput = null;
            } else if (event.event === 'error') {
              status = 'error';
              pendingInput = null;
            } else if (event.event === 'progress') {
              progress = { stage: event.data.stage, current: event.data.current, total: event.data.total };
            }

            return {
              timelineItems: reduceSSEEvent(prev.timelineItems, event),
              status,
              pendingInput,
              progress,
            };
          });

          if (event.event === 'done' || event.event === 'error') {
            get().sessions[threadId]?.eventSourceClose?.();
            patchSession(threadId, { eventSourceClose: null });
          }
        },
        onError: (err) => {
          console.error('SSE Error', err);
          patchSession(threadId, { status: 'error' });
        },
      });

      patchSession(threadId, { eventSourceClose: close });
    },

    replyNeedsInput: async (threadId, runId, replyTo, content) => {
      try {
        patchSession(threadId, { status: 'running', pendingInput: null });
        await runsApi.submitInput(runId, { reply_to: replyTo, content });
      } catch (err) {
        console.error('Failed to submit input', err);
        patchSession(threadId, { status: 'error' });
      }
    },

    cancelRun: async (threadId, runId) => {
      try {
        await runsApi.cancel(runId);
        get().sessions[threadId]?.eventSourceClose?.();
        patchSession(threadId, { status: 'done', eventSourceClose: null });
      } catch (err) {
        console.error('Failed to cancel run', err);
      }
    },

    clearRun: (threadId) => {
      get().sessions[threadId]?.eventSourceClose?.();
      patchSession(threadId, {
        activeRunId: null,
        status: 'idle',
        timelineItems: [],
        pendingInput: null,
        progress: null,
        plan: null,
        eventSourceClose: null,
      });
    },

    // 切 project 时关闭该 project 下所有 thread 的连接（避免离开后仍串事件）。
    closeSessions: (threadIds) => {
      const { sessions } = get();
      threadIds.forEach((id) => sessions[id]?.eventSourceClose?.());
      set((state) => {
        const next = { ...state.sessions };
        threadIds.forEach((id) => {
          if (next[id]) next[id] = { ...next[id], eventSourceClose: null };
        });
        return { sessions: next };
      });
    },
  };
});
