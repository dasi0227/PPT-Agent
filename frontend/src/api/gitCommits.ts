import { commandsApi, type CommandExecution } from './commands';
import { subscribeThreadEvents } from './threadJournal';
import type { GitCommitPhase, GitCommitResult } from './types';

export type GitCommitOutput = (GitCommitResult & { empty?: false }) | { empty: true };
export type GitCommitExecution = CommandExecution<GitCommitOutput>;

export function isGitCommitRunning(status: GitCommitExecution['status']): boolean {
  return status === 'accepted' || status === 'running' || status === 'cancel_requested';
}

export function gitCommitPhase(value: number | undefined): GitCommitPhase | null {
  return value === undefined ? null : (['staging', 'analyzing', 'committing'] as const)[value] ?? null;
}

export function subscribeGitCommitExecution(commandId: string, options: {
  onUpdate: (command: GitCommitExecution) => void;
  onError?: () => void;
}): () => void {
  let stopped = false;
  let unsubscribe = () => {};
  let lastAttempt = 0;
  let lastUpdated = 0;
  let terminal = false;
  const emit = (command: GitCommitExecution) => {
    if (stopped || command.command_id !== commandId || command.kind !== 'commit' ||
      command.attempt_no < lastAttempt ||
      (command.attempt_no === lastAttempt && (command.updated_at < lastUpdated || terminal))) return;
    lastAttempt = command.attempt_no;
    lastUpdated = command.updated_at;
    terminal = !isGitCommitRunning(command.status);
    options.onUpdate(command);
  };
  void commandsApi.get<GitCommitOutput>(commandId).then((command) => {
    if (stopped) return;
    unsubscribe = subscribeThreadEvents(command.thread_id, {
      event: (entry) => {
        if (entry.command_id === commandId && entry.type.startsWith('command.'))
          emit(entry.data as unknown as GitCommitExecution);
      },
      status: (status) => {
        if (status === 'open') void commandsApi.get<GitCommitOutput>(commandId).then(emit).catch(options.onError);
      },
    });
    emit(command);
  }).catch(options.onError);
  return () => { stopped = true; unsubscribe(); };
}
