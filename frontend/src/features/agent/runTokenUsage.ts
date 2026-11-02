import type { SSEEvent, TokenUsage } from '../../api/types';

export interface RunTokenUsage extends TokenUsage {
  calls: Record<string, TokenUsage | null>;
  unknown_calls: number;
}

export type RunTokenUsageByRun = Record<string, RunTokenUsage>;

// Replayed notifications and overlapping history/live subscriptions share the
// same immutable call identity. Actual retries have their own call IDs.
export function reduceRunTokenUsage(state: RunTokenUsageByRun, event: SSEEvent): RunTokenUsageByRun {
  if (event.event !== 'run.usage.updated') return state;
  return addCallUsage(state, event.data.run_id, event.data.call.call_id, event.data.call.usage);
}

function addCallUsage(state: RunTokenUsageByRun, runId: string, callId: string, usage: TokenUsage | null): RunTokenUsageByRun {
  const current = state[runId];
  if (current && Object.prototype.hasOwnProperty.call(current.calls, callId)) return state;
  return {
    ...state,
    [runId]: {
      calls: { ...current?.calls, [callId]: usage },
      input_tokens: (current?.input_tokens ?? 0) + (usage?.input_tokens ?? 0),
      output_tokens: (current?.output_tokens ?? 0) + (usage?.output_tokens ?? 0),
      total_tokens: (current?.total_tokens ?? 0) + (usage?.total_tokens ?? 0),
      unknown_calls: (current?.unknown_calls ?? 0) + (usage === null ? 1 : 0),
    },
  };
}

// History may resolve after newer live events. Merge immutable call records,
// including overlapping snapshots, instead of replacing or adding aggregates.
export function mergeRunTokenUsage(current: RunTokenUsageByRun, incoming: RunTokenUsageByRun): RunTokenUsageByRun {
  let merged = current;
  for (const [runId, usage] of Object.entries(incoming)) {
    for (const [callId, call] of Object.entries(usage.calls)) {
      merged = addCallUsage(merged, runId, callId, call);
    }
  }
  return merged;
}

export function formatRunTokenUsage(usage: RunTokenUsage | undefined): string | null {
  if (!usage || usage.unknown_calls === Object.keys(usage.calls).length) return null;
  const total = usage.total_tokens;
  if (total < 1_000) return String(total);
  if (total < 1_000_000) return `${(total / 1_000).toFixed(2)}K`;
  return `${(total / 1_000_000).toFixed(2)}M`;
}
