import type {
  PlanState,
  RunMode,
  RunScope,
  Skill,
} from '../../api/types';
import { parsePublicEvent } from '../../api/sse';
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

function readHistorySkills(data: Record<string, unknown>): Skill[] {
  if (!Array.isArray(data.skills)) return [];
  return data.skills.slice(0, 3).flatMap((value) => {
    if (!isRecord(value) || typeof value.id !== 'string' || typeof value.name !== 'string' ||
      typeof value.description !== 'string') return [];
    return [{
      id: value.id,
      name: value.name,
      description: value.description,
      ...(typeof value.local_path === 'string' ? { local_path: value.local_path } : {}),
      ...(typeof value.open_url === 'string' ? { open_url: value.open_url } : {}),
    }];
  });
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
        skills: readHistorySkills(entry.data),
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
    if (entry.type === 'git.commit.completed') {
      const commit = isRecord(entry.data.commit) ? entry.data.commit : null;
      if (!commit || typeof commit.title !== 'string' || typeof commit.committed_at !== 'string') continue;
      items.push({
        id: `git-commit:${String(entry.data.operation_id ?? entry.run_id)}`,
        type: 'git_commit',
        operationId: String(entry.data.operation_id ?? entry.run_id),
        status: 'completed',
        title: commit.title,
        items: Array.isArray(commit.items) ? commit.items.filter((item: unknown): item is string => typeof item === 'string') : [],
        branch: String(commit.branch ?? ''),
        hash: String(commit.hash ?? ''),
        filesChanged: Number(commit.files_changed ?? 0),
        insertions: Number(commit.insertions ?? 0),
        deletions: Number(commit.deletions ?? 0),
        timestamp: Date.parse(commit.committed_at) || (entry.ts || 0) * 1000,
      });
      continue;
    }
    if (entry.type === 'git.commit.failed') {
      const error = isRecord(entry.data.error) ? entry.data.error : null;
      items.push({
        id: `git-commit:${String(entry.data.operation_id ?? entry.run_id)}`,
        type: 'git_commit',
        operationId: String(entry.data.operation_id ?? entry.run_id),
        status: 'failed',
        retryable: error?.retryable === true,
        timestamp: Date.parse(String(entry.data.occurred_at ?? '')) || (entry.ts || 0) * 1000,
      });
      continue;
    }
    const event = parsePublicEvent(entry.type, entry.data, String(entry.seq));
    if (!event) continue;
    items = reduceSSEEvent(items, event);
    plan = reducePlan(plan, event);
    if (entry.run_id !== session.activeRunId) {
      session = { activeRunId: entry.run_id, status: 'running', pendingQuestion: null };
    }
    if (event.event === 'question.asked') {
      session = {
        ...session,
        status: 'waiting',
        pendingQuestion: { id: event.data.question_id, prompt: event.data.questions[0].title },
      };
    } else if (event.event === 'question.answered') {
      session = { ...session, status: 'running', pendingQuestion: null };
    } else if (event.event === 'plan.approval_requested') {
      session = { ...session, status: 'waiting', pendingQuestion: null };
    } else if (event.event === 'plan.approval_answered') {
      session = { ...session, status: 'running', pendingQuestion: null };
    } else if (event.event === 'command.permission_requested') {
      session = { ...session, status: 'waiting', pendingQuestion: null };
    } else if (event.event === 'command.permission_answered') {
      session = { ...session, status: 'running', pendingQuestion: null };
    } else if (event.event === 'run.mode_changed') {
      session = { ...session, status: 'running', mode: event.data.mode, pendingQuestion: null };
    } else if (event.event === 'run.resumed') {
      session = { ...session, status: 'running', pendingQuestion: null };
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
