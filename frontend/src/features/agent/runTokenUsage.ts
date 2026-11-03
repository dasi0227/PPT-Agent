import type { TokenUsage } from '../../api/types';

export interface RunTokenUsage extends TokenUsage { calls: number; unknown_calls: number }
export type RunTokenUsageByRun = Record<string, RunTokenUsage>;

export function formatRunTokenUsage(usage: RunTokenUsage | undefined): string | null {
  if (!usage || usage.unknown_calls === usage.calls) return null;
  const total = usage.total_tokens;
  if (total < 1_000) return String(total);
  if (total < 1_000_000) return `${(total / 1_000).toFixed(2)}K`;
  return `${(total / 1_000_000).toFixed(2)}M`;
}
