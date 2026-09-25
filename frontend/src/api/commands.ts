import { APIError, fetchClient, RequestCanceledError } from './client';
import { subscribeThreadEvents } from './threadJournal';

export type CommandKind = 'rename' | 'polish' | 'kickoff' | 'handoff' | 'compact' | 'commit';
export interface CommandExecution<T = unknown> {
  command_id: string; attempt_id: string; attempt_no: number; thread_id: string; project_id: string;
  kind: CommandKind; source: 'user' | 'automatic'; status: 'accepted' | 'running' | 'cancel_requested' | 'completed' | 'failed' | 'canceled' | 'interrupted';
  previous_title?: string; phase: number; input: Record<string, unknown>; result?: T;
  error?: { code: string; message: string; retryable?: boolean };
  latest_success?: CommandExecution<T>; created_at: number; updated_at: number;
}
const known = new Map<string, CommandExecution>();
export const commandsApi = {
  get: <T = unknown>(id: string) => fetchClient<CommandExecution<T>>(`/commands/${encodeURIComponent(id)}`, { reportError: false }),
  create: <T = unknown>(threadId: string, kind: CommandKind, input: unknown, options: { commandId?: string; baseAttemptId?: string; feedback?: string; requestKey?: string } = {}) =>
    fetchClient<CommandExecution<T>>(`/threads/${threadId}/commands`, { method: 'POST', reportError: false, body: JSON.stringify({
      request_key: options.requestKey ?? crypto.randomUUID(), kind, input, command_id: options.commandId,
      base_attempt_id: options.baseAttemptId, feedback: options.feedback,
    }) }),
  cancel: (id: string, attemptId: string) => fetchClient<CommandExecution>(`/commands/${encodeURIComponent(id)}/cancel`, {
    method: 'POST', reportError: false, body: JSON.stringify({ attempt_id: attemptId, request_key: crypto.randomUUID() }),
  }),
};
export async function cancelPersistedCommand(id: string): Promise<void> {
  const command = await commandsApi.get(id);
  await commandsApi.cancel(command.command_id, command.attempt_id);
}
export async function runCommand<T>(threadId: string, kind: CommandKind, input: unknown, options: {
  commandId?: string; feedback?: string; signal?: AbortSignal; onProgress?: (phase: number) => void;
} = {}): Promise<T> {
  let previous = options.commandId ? known.get(options.commandId) : undefined;
  if (!previous && options.commandId) {
    try { previous = await commandsApi.get(options.commandId); }
    catch (error) { if (!(error instanceof APIError) || error.status !== 404) throw error; }
  }
  const accepted = await commandsApi.create<T>(threadId, kind, input, {
    commandId: options.commandId, feedback: options.feedback,
    baseAttemptId: previous?.latest_success?.attempt_id ?? (previous?.status === 'completed' ? previous.attempt_id : undefined),
  });
  known.set(accepted.command_id, accepted);
  return new Promise<T>((resolve, reject) => {
    let settled = false; let stop = () => {};
    const finish = (command: CommandExecution<T>) => {
      if (settled || command.attempt_id !== accepted.attempt_id) return;
      known.set(command.command_id, command); options.onProgress?.(command.phase);
      if (['accepted', 'running', 'cancel_requested'].includes(command.status)) return;
      settled = true; stop(); options.signal?.removeEventListener('abort', cancel);
      if (command.status === 'completed') resolve(command.result as T);
      else if (command.status === 'canceled') reject(new RequestCanceledError());
      else reject(new Error(command.error?.message ?? '命令已中断，请重试'));
    };
    const refresh = () => { void commandsApi.get<T>(accepted.command_id).then(finish).catch(() => {}); };
    const cancel = () => { void commandsApi.cancel(accepted.command_id, accepted.attempt_id).then(refresh).catch((error) => { if (settled) return; settled = true; stop(); options.signal?.removeEventListener('abort', cancel); reject(error); }); };
    stop = subscribeThreadEvents(threadId, {
      event: (event) => { if (event.command_id === accepted.command_id && event.attempt_id === accepted.attempt_id && event.type.startsWith('command.')) finish(event.data as unknown as CommandExecution<T>); },
      status: (status) => { if (status === 'open') refresh(); },
      reset: refresh,
    });
    options.signal?.addEventListener('abort', cancel, { once: true });
    if (options.signal?.aborted) cancel();
    finish(accepted);
    if (!settled) refresh();
  });
}
