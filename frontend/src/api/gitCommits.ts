import type { CommandExecution } from './commands';
import type { GitCommitPhase, GitCommitResult } from './types';

export type GitCommitOutput = (GitCommitResult & { empty?: false }) | { empty: true };
export type GitCommitExecution = CommandExecution<GitCommitOutput>;

export function isGitCommitRunning(status: GitCommitExecution['status']): boolean {
  return status === 'accepted' || status === 'running' || status === 'cancel_requested';
}

export function gitCommitPhase(value: number | undefined): GitCommitPhase | null {
  return value === undefined ? null : (['staging', 'analyzing', 'committing'] as const)[value] ?? null;
}
