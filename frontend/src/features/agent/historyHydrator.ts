import type {
  PlanState,
  RunInteraction,
  RunTarget,
  SSEEvent,
  SSEEventName,
} from '../../api/types';
import { reducePlan, reduceSSEEvent, type TimelineItem } from './eventReducer';

export interface HistoryEntry {
  seq: number;
  ts: number;
  run_id: string;
  turn: 'user' | 'agent';
  type: string;
  data: Record<string, unknown>;
}

export interface HydratedRunView {
  items: TimelineItem[];
  plan: PlanState | null;
  session: HistorySessionState;
  lastEventId?: string;
}

export interface HistorySessionState {
  activeRunId: string | null;
  status: 'idle' | 'running' | 'waiting' | 'done' | 'error' | 'canceled';
  target?: RunTarget;
  interaction?: RunInteraction;
  pendingQuestion: { id: string; prompt: string } | null;
}

const publicHistoryEvents = new Set<SSEEventName>([
  'plan.updated',
  'message.reasoning',
  'message.milestone',
  'message.final',
  'tool.started',
  'tool.completed',
  'question.asked',
  'question.answered',
  'run.finished',
]);

export function hydrateRunFromHistory(entries: HistoryEntry[] | unknown): HydratedRunView {
  const emptySession: HistorySessionState = {
    activeRunId: null,
    status: 'idle',
    pendingQuestion: null,
  };
  if (!Array.isArray(entries)) return { items: [], plan: null, session: emptySession };
  // Thread History is append-ordered. Public seq is only monotonic within one
  // Run, so sorting a multi-Run thread by seq would interleave separate turns.
  const ordered = entries.slice();
  let items: TimelineItem[] = [];
  let plan: PlanState | null = null;
  let session = emptySession;

  for (const entry of ordered) {
    if (entry.type === 'user_turn') {
      plan = null;
      session = {
        activeRunId: entry.run_id,
        status: 'running',
        target: entry.data.target as RunTarget | undefined,
        interaction: entry.data.interaction as RunInteraction | undefined,
        pendingQuestion: null,
      };
      items.push({
        id: `${entry.run_id}:${entry.seq}`,
        type: 'user_turn',
        runId: entry.run_id,
        text: String(entry.data.text ?? ''),
        timestamp: (entry.ts || 0) * 1000,
        target: entry.data.target as RunTarget | undefined,
        interaction: entry.data.interaction as RunInteraction | undefined,
      });
      continue;
    }
    if (entry.type === 'steering') {
      const clientMessageId = String(entry.data.client_message_id ?? '');
      items.push({
        id: `steering_${clientMessageId}`,
        type: 'user_turn',
        runId: entry.run_id,
        text: String(entry.data.text ?? ''),
        clientMessageId,
        deliveryStatus: entry.data.status === 'rejected' ? 'rejected' : 'accepted',
        rejectionCode: String(entry.data.rejection_code ?? ''),
        timestamp: (entry.ts || 0) * 1000,
      });
      continue;
    }
    if (!publicHistoryEvents.has(entry.type as SSEEventName)) continue;
    const event = {
      id: String(entry.seq),
      event: entry.type as SSEEventName,
      data: entry.data,
    } as SSEEvent;
    items = reduceSSEEvent(items, event);
    plan = reducePlan(plan, event);
    if (entry.run_id !== session.activeRunId) {
      session = { activeRunId: entry.run_id, status: 'running', pendingQuestion: null };
    }
    if (event.event === 'question.asked') {
      session = {
        ...session,
        status: 'waiting',
        pendingQuestion: { id: event.data.question_id, prompt: event.data.questions?.[0]?.title ?? event.data.prompt },
      };
    } else if (event.event === 'question.answered') {
      session = { ...session, status: 'running', pendingQuestion: null };
    } else if (event.event === 'run.finished') {
      session = {
        ...session,
        status: event.data.status === 'completed'
          ? 'done'
          : event.data.status === 'canceled'
            ? 'canceled'
            : 'error',
        pendingQuestion: null,
      };
    }
  }
  return {
    items,
    plan,
    session,
    lastEventId: [...ordered].reverse().find((entry) =>
      entry.run_id === session.activeRunId && entry.type !== 'steering')?.seq.toString(),
  };
}

export function hydrateFromHistory(entries: HistoryEntry[] | unknown): TimelineItem[] {
  return hydrateRunFromHistory(entries).items;
}
