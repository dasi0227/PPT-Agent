import { APIError, fetchClient, RequestCanceledError } from './client';
import { sessionCanWrite, useSessionStore } from '../stores/sessionStore';
import { compareRevision, parseCommitReceipt, parseSessionEntity, type CommandView, type CommitReceipt } from './session';
import type { Thread } from './types';
import { isThreadDeleted } from '../stores/deletedTargets';

export type CommandKind = 'rename' | 'polish' | 'handoff' | 'compact' | 'commit' | 'suggest';
export interface CommandExecution<T = unknown> {
  scene_revision: string; entity_revision: string; updated_seq: string;
  run_id?: string; command_id: string; attempt_id: string; attempt_no: number; thread_id: string; project_id: string;
  kind: CommandKind; source: 'user' | 'automatic'; status: 'accepted' | 'running' | 'cancel_requested' | 'completed' | 'failed' | 'canceled' | 'interrupted';
  previous_title?: string; phase: number; input: Record<string, unknown>; result?: T;
  error?: { code: string; message: string; retryable?: boolean };
  latest_success?: CommandExecution<T>; created_at: number; updated_at: number;
}
const known = new Map<string, CommandExecution>();
const observers = new Map<string, Set<() => void>>();
interface CreateOptions {
  sceneRevision: string; entityRevision: string; commandId?: string; baseAttemptId?: string; feedback?: string; requestKey: string;
}
const pendingCreates = new Map<string, { threadId: string; signature: string; input: unknown; options: CreateOptions }>();
const pendingCancels = new Map<string, { command: CommandExecution | CommandView; requestKey: string }>();
const activeStatuses = new Set(['accepted', 'running', 'cancel_requested']);
function definitiveFailure(error: unknown): boolean {
  return error instanceof APIError && error.status >= 400 && error.status < 500 && error.status !== 408 && error.status !== 429;
}
export function dropDeletedCommands(threadIds: string[]) {
  for (const threadId of threadIds) {
    observers.get(threadId)?.forEach(stop => stop());
    observers.delete(threadId);
  }
  const removed = new Set(threadIds);
  for (const [id, command] of known) if (removed.has(command.thread_id)) known.delete(id);
  for (const [id, request] of pendingCreates) if (removed.has(request.threadId)) pendingCreates.delete(id);
  for (const [id, request] of pendingCancels) if (removed.has(request.command.thread_id)) pendingCancels.delete(id);
  useSessionStore.getState().drop(threadIds);
}
interface CommandMutation<T = unknown> { command: CommandExecution<T>; receipt: CommitReceipt }
function mutation<T>(raw: unknown, threadId?: string): CommandMutation<T> {
  if (!raw || typeof raw !== 'object') throw new Error('Invalid command response');
  const value = raw as CommandMutation<T>;
  const command = parseSessionEntity(value.command, 'command', threadId ?? value.command.thread_id);
  const receipt = parseCommitReceipt(value.receipt);
  if (command.scene_revision !== receipt.scene_revision) throw new Error('Invalid command scene');
  return { command: command as CommandExecution<T>, receipt };
}
export const commandsApi = {
  get: <T = unknown>(id: string, signal?: AbortSignal) => fetchClient<CommandExecution<T>>(`/commands/${encodeURIComponent(id)}`, { signal, reportError: false }),
  create: async <T = unknown>(threadId: string, kind: CommandKind, input: unknown, options: CreateOptions) => mutation<T>(await fetchClient<unknown>(`/threads/${threadId}/commands`, { method: 'POST', reportError: false, body: JSON.stringify({
    scene_revision: options.sceneRevision, entity_revision: options.entityRevision, request_key: options.requestKey,
    kind, input, command_id: options.commandId, base_attempt_id: options.baseAttemptId, feedback: options.feedback,
  }) }), threadId),
  cancel: async (command: Pick<CommandExecution, 'command_id' | 'attempt_id' | 'scene_revision' | 'entity_revision' | 'thread_id'>,
    requestKey: string = crypto.randomUUID()) => mutation(await fetchClient<unknown>(`/commands/${encodeURIComponent(command.command_id)}/cancel`, {
      method: 'POST', reportError: false, body: JSON.stringify({ attempt_id: command.attempt_id,
        scene_revision: command.scene_revision, entity_revision: command.entity_revision, request_key: requestKey }),
    }), command.thread_id),
};
function currentCommand(id: string): CommandView | CommandExecution | undefined {
  const current = Object.values(useSessionStore.getState().sessions).map(session => session.commandsById[id]).find(Boolean);
  const saved = known.get(id);
  return current && (!saved || current.scene_revision !== saved.scene_revision || compareRevision(current.entity_revision, saved.entity_revision) >= 0) ? current : saved;
}
export async function cancelPersistedCommand(id: string): Promise<void> {
  const command = currentCommand(id);
  const session = command && useSessionStore.getState().sessions[command.thread_id];
  if (!command || !sessionCanWrite(session) || session.sceneRevision !== command.scene_revision) throw new RequestCanceledError();
  let pending = pendingCancels.get(id);
  if (pending && pending.command.scene_revision !== command.scene_revision) {
    pendingCancels.delete(id);
    throw new APIError(409, 'SCENE_CHANGED', 'SCENE_CHANGED');
  }
  if (!pending) {
    if (!activeStatuses.has(command.status) || command.status === 'cancel_requested') return;
    pending = { command: { ...command }, requestKey: crypto.randomUUID() };
    pendingCancels.set(id, pending);
  }
  try {
    await commandsApi.cancel(pending.command, pending.requestKey);
    if (pendingCancels.get(id) === pending) pendingCancels.delete(id);
  } catch (error) {
    if (definitiveFailure(error) && pendingCancels.get(id) === pending) pendingCancels.delete(id);
    throw error;
  }
  await useSessionStore.getState().reconnect(command.thread_id);
}
async function commandSession(threadId: string) {
  const store = useSessionStore.getState();
  let projectId = store.sessions[threadId]?.projectId;
  if (!projectId) projectId = (await fetchClient<Thread>(`/threads/${encodeURIComponent(threadId)}`, { reportError: false })).project_id;
  await store.ensure(threadId, projectId);
  const session = useSessionStore.getState().sessions[threadId];
  if (!sessionCanWrite(session)) throw session?.error ?? new RequestCanceledError();
  return session;
}
export async function runCommand<T>(threadId: string, kind: CommandKind, input: unknown, options: {
  commandId?: string; feedback?: string; signal?: AbortSignal; onProgress?: (phase: number) => void;
} = {}): Promise<T> {
  if (isThreadDeleted(threadId) || options.signal?.aborted) throw new RequestCanceledError();
  const start = await commandSession(threadId);
  const commandId = options.commandId ?? `${kind}:${crypto.randomUUID()}`;
  let pending = pendingCreates.get(commandId);
  if (pending && pending.options.sceneRevision !== start.sceneRevision) {
    pendingCreates.delete(commandId);
    throw new APIError(409, 'SCENE_CHANGED', 'SCENE_CHANGED');
  }
  let previous = options.commandId ? currentCommand(options.commandId) : undefined;
  if (!pending && !previous && options.commandId) {
    try { previous = await commandsApi.get(options.commandId); }
    catch (error) { if (!(error instanceof APIError) || error.status !== 404) throw error; }
  }
  if (isThreadDeleted(threadId)) throw new RequestCanceledError();
  if (previous && previous.thread_id !== threadId) throw new APIError(409, 'IDEMPOTENCY_CONFLICT', 'IDEMPOTENCY_CONFLICT');
  if (previous && previous.scene_revision !== start.sceneRevision) throw new APIError(409, 'SCENE_CHANGED', 'SCENE_CHANGED');
  let accepted: CommandExecution<T>;
  if (!pending && previous && activeStatuses.has(previous.status)) {
    // A retry after acceptance observes the same attempt instead of submitting
    // another attempt while the first worker is still running.
    if (previous.kind !== kind) throw new APIError(409, 'IDEMPOTENCY_CONFLICT', 'IDEMPOTENCY_CONFLICT');
    accepted = await commandsApi.get<T>(previous.command_id);
  } else {
    const signature = JSON.stringify({ kind, input, feedback: options.feedback });
    if (pending && (pending.threadId !== threadId || pending.signature !== signature))
      throw new APIError(409, 'IDEMPOTENCY_CONFLICT', 'IDEMPOTENCY_CONFLICT');
    if (!pending) {
      pending = { threadId, signature, input: JSON.parse(JSON.stringify(input)) as unknown, options: {
        sceneRevision: start.sceneRevision, entityRevision: previous?.entity_revision ?? '0', requestKey: crypto.randomUUID(),
        commandId, feedback: options.feedback,
        baseAttemptId: previous?.latest_success?.attempt_id ?? (previous?.status === 'completed' ? previous.attempt_id : undefined),
      } };
      pendingCreates.set(commandId, pending);
    }
    try {
      const response = await commandsApi.create<T>(threadId, kind, pending.input, pending.options);
      accepted = response.command;
      if (pendingCreates.get(commandId) === pending) pendingCreates.delete(commandId);
    } catch (error) {
      if (definitiveFailure(error) && pendingCreates.get(commandId) === pending) pendingCreates.delete(commandId);
      throw error;
    }
  }
  if (isThreadDeleted(threadId)) throw new RequestCanceledError();
  const current = useSessionStore.getState().sessions[threadId];
  if (current?.sceneRevision !== start.sceneRevision || current.generation !== start.generation) throw new RequestCanceledError();
  known.set(accepted.command_id, accepted);
  return new Promise<T>((resolve, reject) => {
    let settled = false; let stop = () => {};
    const statusController = new AbortController();
    let pollTimer: ReturnType<typeof setTimeout> | undefined;
    const watching = observers.get(threadId) ?? new Set<() => void>();
    observers.set(threadId, watching);
    const dispose = () => {
      if (settled) return;
      settled = true; cleanup();
      reject(new RequestCanceledError());
    };
    watching.add(dispose);
    const cleanup = () => {
      clearTimeout(pollTimer);
      statusController.abort();
      stop(); options.signal?.removeEventListener('abort', cancel);
      watching.delete(dispose);
      if (watching.size === 0 && observers.get(threadId) === watching) observers.delete(threadId);
    };
    const finish = (command: CommandExecution<T>) => {
      if (settled) return;
      if (command.attempt_id !== accepted.attempt_id || command.scene_revision !== accepted.scene_revision
        || command.thread_id !== threadId) { dispose(); return; }
      const source = useSessionStore.getState().sessions[threadId];
      if (!sessionCanWrite(source) || source.sceneRevision !== accepted.scene_revision) { dispose(); return; }
      const projected = source.commandsById[accepted.command_id];
      if (projected?.attempt_id === command.attempt_id && compareRevision(projected.entity_revision, command.entity_revision) > 0) return;
      known.set(command.command_id, command); options.onProgress?.(command.phase);
      if (['accepted', 'running', 'cancel_requested'].includes(command.status)) return;
      settled = true; cleanup();
      if (command.status === 'completed') resolve(command.result as T);
      else if (command.status === 'canceled') reject(new RequestCanceledError());
      else reject(new Error(command.error?.message ?? '命令已中断，请重试'));
    };
    let refreshing: Promise<void> | undefined;
    const refresh = () => {
      if (settled || refreshing) return;
      clearTimeout(pollTimer);
      refreshing = commandsApi.get<T>(accepted.command_id, statusController.signal).then(finish).catch(error => {
        if (settled) return;
        settled = true; cleanup(); reject(error);
      }).finally(() => {
        refreshing = undefined;
        if (!settled) pollTimer = setTimeout(refresh, 2000);
      });
    };
    const cancel = () => { void cancelPersistedCommand(accepted.command_id).then(refresh).catch((error) => { if (settled) return; settled = true; cleanup(); reject(error); }); };
    // All command kinds share the same committed session state and connection.
    // Sparse status reads also surface a failed terminal commit, which cannot
    // produce a session update. They never resubmit or execute the command.
    let observed: CommandView | undefined;
    const observe = () => {
      const session = useSessionStore.getState().sessions[threadId];
      if (session?.sceneRevision !== accepted.scene_revision) { dispose(); return; }
      const command = session?.commandsById[accepted.command_id];
      if (!command || command === observed || command.attempt_id !== accepted.attempt_id) return;
      observed = command;
      if (command.content_ref) {
        options.onProgress?.(command.phase);
        if (!['accepted', 'running', 'cancel_requested'].includes(command.status)) refresh();
      } else finish(command as CommandExecution<T>);
    };
    const release = useSessionStore.getState().retain(threadId, accepted.project_id);
    const unsubscribe = useSessionStore.subscribe(observe);
    stop = () => { unsubscribe(); release(); };
    options.signal?.addEventListener('abort', cancel, { once: true });
    if (options.signal?.aborted) cancel();
    finish(accepted);
    if (!settled) { observe(); refresh(); }
  });
}
