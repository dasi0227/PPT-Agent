import type {
  PlanState,
  RunMode,
  RunScope,
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
  scope?: RunScope;
  mode?: RunMode;
  pendingQuestion: { id: string; prompt: string } | null;
}

const publicHistoryEvents = new Set<SSEEventName>([
  'plan.updated',
  'plan.approval_requested',
  'plan.approval_answered',
  'run.mode_changed',
  'message.reasoning',
  'message.milestone',
  'message.final',
  'tool.started',
  'tool.completed',
  'question.asked',
  'question.answered',
  'run.completed',
  'run.failed',
  'run.error',
  'run.canceled',
]);

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object' && !Array.isArray(value);
}

function readHistoryScope(data: Record<string, unknown>): RunScope | undefined {
  const raw = data.scope;
  if (!isRecord(raw)) return undefined;
  const artifact = raw.artifact;
  if ((artifact !== 'spec' && artifact !== 'ppt') ||
    (raw.level !== 'slide' && raw.level !== 'deck')) return undefined;
  return {
    artifact,
    level: raw.level,
    ...(typeof raw.slide_id === 'string' && raw.slide_id !== '' ? { slide_id: raw.slide_id } : {}),
  };
}

function readHistoryIntent(data: Record<string, unknown>): RunMode | undefined {
  const mode = data.mode;
  return mode === 'talk' || mode === 'ask' || mode === 'plan' || mode === 'execute'
    ? mode
    : undefined;
}

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
      const scope = readHistoryScope(entry.data);
      const mode = readHistoryIntent(entry.data);
      if (!scope || !mode) continue;
      plan = null;
      session = {
        activeRunId: entry.run_id,
        status: 'running',
        scope,
        mode,
        pendingQuestion: null,
      };
      items.push({
        id: `${entry.run_id}:${entry.seq}`,
        type: 'user_turn',
        runId: entry.run_id,
        text: String(entry.data.text ?? ''),
        timestamp: (entry.ts || 0) * 1000,
        scope,
        mode,
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
    if (entry.data.schema_version !== 3) continue;
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
    } else if (event.event === 'plan.approval_requested') {
      session = { ...session, status: 'waiting', pendingQuestion: null };
    } else if (event.event === 'plan.approval_answered') {
      session = { ...session, status: 'running', pendingQuestion: null };
    } else if (event.event === 'run.mode_changed') {
      session = { ...session, status: 'running', mode: event.data.mode, pendingQuestion: null };
    } else if (event.event === 'run.completed') {
      session = {
        ...session,
        status: 'done',
        pendingQuestion: null,
      };
    } else if (event.event === 'run.canceled') {
      session = {
        ...session,
        status: 'canceled',
        pendingQuestion: null,
      };
    } else if (event.event === 'run.failed' || event.event === 'run.error') {
      session = {
        ...session,
        status: 'error',
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
