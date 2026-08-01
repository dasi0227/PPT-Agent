import type {
  ExecutionStrategy,
  PlanState,
  RunInteraction,
  RunTarget,
  SSEEvent,
  SSEEventName,
  StructuredOutcome,
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
  strategy: ExecutionStrategy | null;
}

const runtimeEvents = new Set<SSEEventName>([
  'run.started', 'context.assembled', 'strategy.selected', 'plan.created',
  'stage.started', 'stage.completed', 'step.started', 'step.completed', 'step.failed',
  'tool.called', 'tool.completed', 'verification.completed',
  'repair.started', 'repair.completed', 'artifact.staged', 'artifact.committed',
  'status.summary', 'needs_input', 'run.completed', 'run.failed', 'run.canceled',
]);

export function hydrateRunFromHistory(entries: HistoryEntry[] | unknown): HydratedRunView {
  if (!Array.isArray(entries)) return { items: [], plan: null, strategy: null };
  const sorted = entries.slice().sort((left, right) => left.seq - right.seq);
  let items: TimelineItem[] = [];
  let plan: PlanState | null = null;
  let strategy: ExecutionStrategy | null = null;

  for (const entry of sorted) {
    const timestamp = (entry.ts || 0) * 1000;
    const id = `hist_${entry.seq}`;
    if (entry.type === 'user_turn') {
      items.push({
        id, type: 'user_turn', text: String(entry.data.text ?? ''), timestamp,
        target: entry.data.target as RunTarget | undefined,
        interaction: entry.data.interaction as RunInteraction | undefined,
      });
      continue;
    }
    if (entry.type === 'context_assembled') {
      items.push({
        id, type: 'context_status', profile: String(entry.data.profile ?? ''),
        warnings: Array.isArray(entry.data.warnings) ? entry.data.warnings.map(String) : [],
        readOnly: Boolean(entry.data.read_only), timestamp,
      });
      continue;
    }
    if (entry.type === 'markdown') {
      items.push({ id, type: 'markdown', text: String(entry.data.text ?? ''), timestamp });
      continue;
    }
    if (entry.type === 'final_result') {
      const result = entry.data.result;
      items.push({
        id,
        type: 'final_result',
        result: typeof result === 'string' || result === null
          ? result
          : typeof result === 'object'
            ? result as StructuredOutcome
            : null,
        timestamp,
      });
      continue;
    }
    if (entry.type === 'error') {
      items.push({
        id, type: 'error', code: entry.data.code,
        message: String(entry.data.message ?? entry.data.status ?? 'Run failed'), timestamp,
      });
      continue;
    }
    if (!runtimeEvents.has(entry.type as SSEEventName)) continue;
    const event = { id, event: entry.type as SSEEventName, data: entry.data } as SSEEvent;
    items = reduceSSEEvent(items, event);
    plan = reducePlan(plan, event);
    if (entry.type === 'strategy.selected') {
      strategy = entry.data.strategy as ExecutionStrategy;
    }
  }
  return { items, plan, strategy };
}

export function hydrateFromHistory(entries: HistoryEntry[] | unknown): TimelineItem[] {
  return hydrateRunFromHistory(entries).items;
}
