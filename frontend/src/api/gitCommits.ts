import { fetchClient } from './client';
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

export function subscribeGitCommitEvents(
  operationId: string,
  options: GitCommitSSEOptions,
): () => void {
  const url = new URL(`/api/v1/git-commits/${operationId}/events`, window.location.origin);
  if (options.lastEventId) url.searchParams.set('last_event_id', options.lastEventId);
  const source = new EventSource(url.toString());
  const handle = (message: MessageEvent) => {
    const event = parseGitCommitEvent(message.type, String(message.data), message.lastEventId || undefined);
    if (event) options.onMessage(event);
  };
  eventNames.forEach((name) => source.addEventListener(name, handle));
  source.onerror = () => options.onError?.();
  return () => source.close();
}

export const gitCommitsApi = {
  create: (projectId: string, body: {
    thread_id: string;
    model: string;
    client_request_id: string;
  }) => fetchClient<GitCommitOperation>(`/projects/${projectId}/git-commits`, {
    method: 'POST',
    body: JSON.stringify(body),
  }),
  get: (operationId: string) => fetchClient<GitCommitOperation>(`/git-commits/${operationId}`),
};

export function isGitCommitRunning(status: GitCommitOperation['status']): boolean {
  return status === 'accepted' || status === 'running';
}

export function gitCommitPhase(value: string | undefined): GitCommitPhase | null {
  return value === 'staging' || value === 'analyzing' || value === 'committing' ? value : null;
}
