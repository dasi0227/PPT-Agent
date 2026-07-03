import { create } from 'zustand';
import { runsApi } from '../api/runs';
import { subscribeRunEvents } from '../api/sse';
import { RunPayload } from '../api/types';
import { TimelineItem, reduceSSEEvent } from '../features/agent/eventReducer';

interface RunState {
  activeRunId: string | null;
  status: 'idle' | 'running' | 'done' | 'error' | 'needs_input';
  mode: string;
  scope: string;
  timelineItems: TimelineItem[];
  pendingInput: { id: string, prompt: string, choices?: string[] } | null;
  eventSourceClose: (() => void) | null;
  progress: { stage: string, current: number, total: number } | null;

  createRun: (threadId: string, payload: RunPayload) => Promise<void>;
  subscribeRun: (runId: string, lastEventId?: string) => void;
  replyNeedsInput: (runId: string, replyTo: string, content: string) => Promise<void>;
  cancelRun: (runId: string) => Promise<void>;
  clearRun: () => void;
}

export const useRunStore = create<RunState>((set, get) => ({
  activeRunId: null,
  status: 'idle',
  mode: 'normal',
  scope: '/current',
  timelineItems: [],
  pendingInput: null,
  eventSourceClose: null,
  progress: null,

  createRun: async (threadId, payload) => {
    try {
      // clear previous
      const existingClose = get().eventSourceClose;
      if (existingClose) existingClose();
      
      set({ 
        activeRunId: null, 
        status: 'running', 
        timelineItems: [], 
        pendingInput: null,
        mode: payload.mode,
        scope: payload.scope,
        progress: null
      });

      const run = await runsApi.create(threadId, payload);
      set({ activeRunId: run.id });
      get().subscribeRun(run.id);
    } catch (err) {
      set({ status: 'error' });
      console.error(err);
    }
  },

  subscribeRun: (runId, lastEventId) => {
    const existingClose = get().eventSourceClose;
    if (existingClose) existingClose();

    const close = subscribeRunEvents(runId, {
      lastEventId,
      onMessage: (event) => {
        set((state) => {
          let newStatus = state.status;
          let newPendingInput = state.pendingInput;
          let newProgress = state.progress;

          if (event.event === 'needs_input') {
            newStatus = 'needs_input';
            newPendingInput = { 
              id: event.data.id, 
              prompt: event.data.prompt, 
              choices: event.data.choices 
            };
          } else if (event.event === 'done') {
            newStatus = 'done';
            newPendingInput = null;
          } else if (event.event === 'error') {
            newStatus = 'error';
            newPendingInput = null;
          } else if (event.event === 'progress') {
            newProgress = {
              stage: event.data.stage,
              current: event.data.current,
              total: event.data.total
            };
          }

          return {
            timelineItems: reduceSSEEvent(state.timelineItems, event),
            status: newStatus,
            pendingInput: newPendingInput,
            progress: newProgress
          };
        });

        // Close connection on terminal states
        if (event.event === 'done' || event.event === 'error') {
          get().eventSourceClose?.();
          set({ eventSourceClose: null });
        }
      },
      onError: (err) => {
        console.error('SSE Error', err);
        set({ status: 'error' });
      }
    });

    set({ eventSourceClose: close });
  },

  replyNeedsInput: async (runId, replyTo, content) => {
    try {
      set({ status: 'running', pendingInput: null });
      await runsApi.submitInput(runId, { reply_to: replyTo, content });
    } catch (err) {
      console.error('Failed to submit input', err);
      set({ status: 'error' });
    }
  },

  cancelRun: async (runId) => {
    try {
      await runsApi.cancel(runId);
      const close = get().eventSourceClose;
      if (close) close();
      set({ status: 'done', eventSourceClose: null });
    } catch (err) {
      console.error('Failed to cancel run', err);
    }
  },

  clearRun: () => {
    const close = get().eventSourceClose;
    if (close) close();
    set({
      activeRunId: null,
      status: 'idle',
      timelineItems: [],
      pendingInput: null,
      eventSourceClose: null,
      progress: null
    });
  }
}));
