import { commandsApi, type CommandExecution } from './commands';
import { subscribeThreadEvents } from './threadJournal';
import type {
  GitCommitEvent,
  GitCommitOperation,
  GitCommitPhase,
  GitCommitPublicError,
  GitCommitResult,
} from './types';

const eventNames = [
  'git.commit.progress',
  'git.commit.empty',
  'git.commit.completed',
  'git.commit.failed',
] as const;

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object' && !Array.isArray(value);
}

function validBase(value: Record<string, unknown>): boolean {
  return value.schema_version === 1
    && typeof value.operation_id === 'string'
    && typeof value.project_id === 'string'
    && typeof value.thread_id === 'string'
    && typeof value.occurred_at === 'string'
    && !Number.isNaN(Date.parse(value.occurred_at));
}

function validResult(value: unknown): value is GitCommitResult {
  if (!isRecord(value)) return false;
  return typeof value.title === 'string'
    && Array.isArray(value.items)
    && value.items.every((item) => typeof item === 'string')
    && typeof value.branch === 'string'
    && typeof value.hash === 'string'
    && ['files_changed', 'insertions', 'deletions'].every((key) => (
      typeof value[key] === 'number' && Number.isInteger(value[key]) && Number(value[key]) >= 0
    ))
    && typeof value.committed_at === 'string'
    && !Number.isNaN(Date.parse(value.committed_at));
}

function validError(value: unknown): value is GitCommitPublicError {
  return isRecord(value)
    && typeof value.code === 'string'
    && typeof value.message === 'string'
    && typeof value.retryable === 'boolean';
}

export function parseGitCommitEvent(eventName: string, raw: string, id?: string): GitCommitEvent | null {
  if (!eventNames.includes(eventName as typeof eventNames[number])) return null;
  let data: unknown;
  try {
    data = JSON.parse(raw) as unknown;
  } catch {
    return null;
  }
  if (!isRecord(data) || !validBase(data)) return null;
  if (eventName === 'git.commit.progress') {
    if (!['staging', 'analyzing', 'committing'].includes(String(data.phase))) return null;
  } else if (eventName === 'git.commit.completed') {
    if (!validResult(data.commit)) return null;
  } else if (eventName === 'git.commit.failed' && !validError(data.error)) {
    return null;
  }
  return { id, event: eventName, data } as unknown as GitCommitEvent;
}

export interface GitCommitSSEOptions {
  lastEventId?: string;
  onMessage: (event: GitCommitEvent) => void;
  onError?: () => void;
}

function operationFromCommand(command: CommandExecution<GitCommitResult & { empty?: boolean }>): GitCommitOperation {
  const status = command.status === 'completed' ? (command.result?.empty ? 'empty' : 'completed')
    : command.status === 'accepted' ? 'accepted' : ['running', 'cancel_requested'].includes(command.status) ? 'running' : 'failed';
  return { id: command.command_id, thread_id: command.thread_id, project_id: command.project_id, status,
    phase: (['staging', 'analyzing', 'committing'] as const)[Math.max(0, command.phase)],
    events_url: `/api/v1/threads/${command.thread_id}/events`,
    result: command.result?.empty ? undefined : command.result,
    error: command.error ? { code: command.status === 'canceled' ? 'COMMIT_CANCELED' : command.error.code, message: command.error.message, retryable: command.error.retryable ?? false } : undefined,
  };
}
export function subscribeGitCommitEvents(operationId: string, options: GitCommitSSEOptions): () => void {
  let stopped = false; let unsubscribe = () => {};
  let lastAttempt = 0; let lastUpdated = 0; let terminal = false;
  const emit = (command: CommandExecution<GitCommitResult & { empty?: boolean }>) => {
    if (stopped || command.attempt_no < lastAttempt || (command.attempt_no === lastAttempt && (command.updated_at < lastUpdated || terminal))) return;
    lastAttempt = command.attempt_no; lastUpdated = command.updated_at;
    terminal = !['accepted', 'running', 'cancel_requested'].includes(command.status);
    const operation = operationFromCommand(command);
    const base = { schema_version: 1, operation_id: operationId, project_id: command.project_id, thread_id: command.thread_id, occurred_at: new Date(command.updated_at).toISOString() };
    const name = operation.status === 'completed' ? 'git.commit.completed' : operation.status === 'empty' ? 'git.commit.empty' : operation.status === 'failed' ? 'git.commit.failed' : 'git.commit.progress';
    const data = { ...base, phase: operation.phase, commit: operation.result, error: operation.error };
    const event = parseGitCommitEvent(name, JSON.stringify(data), `${command.attempt_no}:${command.updated_at}`);
    if (event) options.onMessage(event);
  };
  void commandsApi.get<GitCommitResult & { empty?: boolean }>(operationId).then((command) => {
    if (stopped) return;
    unsubscribe = subscribeThreadEvents(command.thread_id, {
      event: (entry) => { if (entry.command_id === operationId && entry.type.startsWith('command.')) emit(entry.data as unknown as CommandExecution<GitCommitResult & { empty?: boolean }>); },
      status: (status) => { if (status === 'open') void commandsApi.get<GitCommitResult & { empty?: boolean }>(operationId).then(emit).catch(options.onError); },
    }); emit(command);
  }).catch(options.onError);
  return () => { stopped = true; unsubscribe(); };
}
export const gitCommitsApi = {
  create: async (_projectId: string, body: { thread_id: string; client_request_id: string; command_id?: string }) =>
    operationFromCommand(await commandsApi.create<GitCommitResult & { empty?: boolean }>(body.thread_id, 'commit', {}, { requestKey: body.client_request_id, commandId: body.command_id })),
  cancel: async (id: string) => {
    const current = await commandsApi.get(id);
    await commandsApi.cancel(id, current.attempt_id);
    return operationFromCommand(await commandsApi.get<GitCommitResult & { empty?: boolean }>(id));
  },
  get: async (id: string) => operationFromCommand(await commandsApi.get<GitCommitResult & { empty?: boolean }>(id)),
};

export function isGitCommitRunning(status: GitCommitOperation['status']): boolean {
  return status === 'accepted' || status === 'running';
}

export function gitCommitPhase(value: string | undefined): GitCommitPhase | null {
  return value === 'staging' || value === 'analyzing' || value === 'committing' ? value : null;
}
