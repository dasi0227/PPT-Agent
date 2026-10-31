import type { SSEEvent, TokenUsage } from '../../api/types';

export interface RunTokenUsage extends TokenUsage {
  calls: Record<string, true>;
  unknown_calls: number;
}

export type RunTokenUsageByRun = Record<string, RunTokenUsage>;

// Replayed notifications and overlapping history/live subscriptions share the
// same immutable call identity. Actual retries have their own call IDs.
export function reduceRunTokenUsage(state: RunTokenUsageByRun, event: SSEEvent): RunTokenUsageByRun {
  if (event.event !== 'run.usage.updated') return state;
  const runId = event.data.run_id;
  const call = event.data.call;
  const current = state[runId];
  if (current && Object.prototype.hasOwnProperty.call(current.calls, call.call_id)) return state;
  return {
    ...state,
    [runId]: {
      calls: { ...current?.calls, [call.call_id]: true },
      input_tokens: (current?.input_tokens ?? 0) + (call.usage?.input_tokens ?? 0),
      output_tokens: (current?.output_tokens ?? 0) + (call.usage?.output_tokens ?? 0),
      total_tokens: (current?.total_tokens ?? 0) + (call.usage?.total_tokens ?? 0),
      unknown_calls: (current?.unknown_calls ?? 0) + (call.usage === null ? 1 : 0),
    },
  };
}

const tokenNumber = new Intl.NumberFormat('en-US', { maximumFractionDigits: 0 });

export function formatRunTokenUsage(usage: RunTokenUsage | undefined): string | null {
  if (!usage || usage.unknown_calls === Object.keys(usage.calls).length) return null;
  return tokenNumber.format(usage.total_tokens);
}
