import { cancelPersistedCommand, dropDeletedCommands } from '../api/commands';
import { currentHistoryEpoch, RequestCanceledError } from '../api/client';
import type {
  BriefingTimelineItem,
  CommandTimelineItem,
  TimelineItem,
} from '../features/agent/eventReducer';
import { IDLE_SESSION, useRunStore } from './runStore';
import { showGlobalError } from './toastStore';
import { isThreadDeleted } from './deletedTargets';

export type CommandStatus = 'loading' | 'completed' | 'failed' | 'canceled';
export interface CommandProgress {
  status: CommandStatus;
  phase?: number;
  cancellable?: boolean;
}
const jobs = new Map<string, { threadId: string; controller: AbortController; cancel: () => void }>();
const retries = new Map<string, { threadId: string; retry: () => void }>();
export function dropThreadCommands(threadIds: string[]) {
  dropDeletedCommands(threadIds);
  const removed = new Set(threadIds);
  for (const [id, job] of jobs) {
    if (!removed.has(job.threadId)) continue;
    jobs.delete(id);
    job.controller.abort();
  }
  for (const [id, saved] of retries) if (removed.has(saved.threadId)) retries.delete(id);
}
export function upsertCommand(threadId: string, item: TimelineItem, replaceId = item.id) {
  if (isThreadDeleted(threadId)) return;
  if (item.type === 'command' && item.kind === 'rename') return;
  useRunStore.setState((state) => {
    const previous = state.sessions[threadId] ?? { ...IDLE_SESSION, processedEventIds: [] };
    const timelineItems = previous.timelineItems.filter(
      (value) => value.id !== item.id || value.id === replaceId,
    );
    const index = timelineItems.findIndex((value) => value.id === replaceId);
    if (index < 0) timelineItems.push(item);
    else timelineItems[index] = item;
    return { sessions: { ...state.sessions, [threadId]: { ...previous, timelineItems } } };
  });
}
export function cancelCommand(id: string) {
  const job = jobs.get(id);
  if (job) job.cancel(); else void cancelPersistedCommand(id).catch((error) => showGlobalError(error instanceof Error ? error.message : '停止命令失败'));
}
export function retryCommand(id: string) {
  retries.get(id)?.retry();
}
export function commandActive(id: string) {
  return jobs.has(id);
}

// Queue only phases received from the service. Minimum dwell is a presentation
// concern; it neither invents backend progress nor delays the server's work.
export async function performCommand<T>(
  threadId: string,
  initial: BriefingTimelineItem | CommandTimelineItem,
  execute: (signal: AbortSignal, phase: (value: number) => void) => Promise<T>,
  complete: (result: T) => TimelineItem,
  retry: () => void,
): Promise<boolean> {
  if (jobs.has(initial.id) || isThreadDeleted(threadId)) return false;
  const controller = new AbortController(),
    epoch = currentHistoryEpoch();
  let live = true,
    current = {
      ...initial,
      timestamp: useRunStore.getState().sessions[threadId]?.timelineItems.find((item) => item.id === initial.id)?.timestamp ?? initial.timestamp,
      phase: -1,
      cancellable: initial.cancellable ?? true,
    };
  let chain = Promise.resolve(),
    paintedAt = 0,
    highest = -1;
  const valid = () =>
    live &&
    !controller.signal.aborted && !isThreadDeleted(threadId) &&
    epoch === currentHistoryEpoch() &&
    Boolean(useRunStore.getState().sessions[threadId]);
  const wait = (ms: number) =>
    new Promise<void>((resolve) => {
      if (controller.signal.aborted) {
        resolve();
        return;
      }
      const done = () => {
        clearTimeout(timer);
        controller.signal.removeEventListener('abort', done);
        resolve();
      };
      const timer = window.setTimeout(done, Math.max(0, ms));
      controller.signal.addEventListener('abort', done, { once: true });
    });
  const settle = (status: 'failed' | 'canceled') => {
    if (!live) return;
    live = false;
    if (!isThreadDeleted(threadId) && epoch === currentHistoryEpoch() && useRunStore.getState().sessions[threadId])
      upsertCommand(threadId, { ...current, status, cancellable: false });
    jobs.delete(initial.id);
  };
  retries.set(initial.id, { threadId, retry });
  jobs.set(initial.id, {
    threadId,
    controller,
    cancel: () => {
      if (!current.cancellable) return;
      current = { ...current, cancellable: false }; upsertCommand(threadId, current);
      controller.abort();
    },
  });
  upsertCommand(threadId, current);
  const phase = (value: number) => {
    if (value <= highest || !valid()) return;
    highest = value;
    chain = chain.then(async () => {
      await wait(450 - (Date.now() - paintedAt));
      if (!valid()) return;
      current = { ...current, phase: value };
      upsertCommand(threadId, current);
      paintedAt = Date.now();
    });
  };
  try {
    const result = await execute(controller.signal, phase);
    if (!valid()) return false;
    // The server has committed the result: cancellation is no longer offered.
    current = { ...current, cancellable: false };
    upsertCommand(threadId, current);
    await chain;
    await wait(450 - (Date.now() - paintedAt));
    if (!valid()) return false;
    upsertCommand(threadId, { ...complete(result), timestamp: current.timestamp }, initial.id);
    live = false;
    retries.delete(initial.id);
    return true;
  } catch (error) {
    settle(
      error instanceof RequestCanceledError ? 'canceled' : 'failed',
    );
    return false;
  } finally {
    if (jobs.get(initial.id)?.controller === controller) jobs.delete(initial.id);
  }
}
