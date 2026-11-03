import { cancelPersistedCommand, dropDeletedCommands } from '../api/commands';
import { currentHistoryEpoch, RequestCanceledError } from '../api/client';
import { showGlobalError } from './toastStore';
import { isThreadDeleted } from './deletedTargets';

export type CommandStatus = 'loading' | 'completed' | 'failed' | 'canceled';
export interface CommandProgress { status: CommandStatus; phase?: number; cancellable?: boolean }
const jobs = new Map<string, { threadId: string; controller: AbortController }>();
const retries = new Map<string, { threadId: string; retry: () => void }>();

export function dropThreadCommands(threadIds: string[]) {
  dropDeletedCommands(threadIds);
  const removed = new Set(threadIds);
  for (const [id, job] of jobs) if (removed.has(job.threadId)) jobs.delete(id);
  for (const [id, retry] of retries) if (removed.has(retry.threadId)) retries.delete(id);
}
export function cancelCommand(id: string) {
  const job = jobs.get(id);
  if (job) job.controller.abort();
  else void cancelPersistedCommand(id).catch(error => showGlobalError(error instanceof Error ? error.message : '停止命令失败'));
}
export function retryCommand(id: string) { retries.get(id)?.retry(); }
export function commandActive(id: string) { return jobs.has(id); }

// This layer owns local actions only. Persisted command status, progress and
// timeline content always arrive through the shared session state.
export async function performCommand<T>(threadId: string, id: string,
  execute: (signal: AbortSignal) => Promise<T>,
  complete: (result: T) => void, retry: () => void): Promise<boolean> {
  if (jobs.has(id) || isThreadDeleted(threadId)) return false;
  const controller = new AbortController(), epoch = currentHistoryEpoch();
  jobs.set(id, { threadId, controller });
  retries.set(id, { threadId, retry });
  try {
    const result = await execute(controller.signal);
    if (isThreadDeleted(threadId) || epoch !== currentHistoryEpoch()) return false;
    // Keep caller-owned effects (e.g. apply generated input) after durable success.
    complete(result);
    retries.delete(id);
    return true;
  } catch (error) {
    if (!(error instanceof RequestCanceledError)) showGlobalError(error instanceof Error ? error.message : '请求失败，请重试');
    return false;
  } finally {
    if (jobs.get(id)?.controller === controller) jobs.delete(id);
  }
}
