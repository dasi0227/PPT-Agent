import { currentHistoryEpoch, RequestCanceledError } from '../api/client';
import type {
  BriefingTimelineItem,
  CommandTimelineItem,
  TimelineItem,
} from '../features/agent/eventReducer';
import { IDLE_SESSION, useRunStore } from './runStore';
import { assertProjectSourcesSaved } from './sourceEditorStore';
import { useProjectStore } from './projectStore';
import { showGlobalWarning } from './toastStore';

export type CommandStatus = 'loading' | 'completed' | 'failed' | 'canceled';
export interface CommandProgress {
  status: CommandStatus;
  phase?: number;
  cancellable?: boolean;
}
const jobs = new Map<string, { controller: AbortController; cancel: () => void }>();
const retries = new Map<string, () => void>();
export function upsertCommand(threadId: string, item: TimelineItem, replaceId = item.id) {
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
  jobs.get(id)?.cancel();
}
export function retryCommand(id: string) {
  retries.get(id)?.();
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
  if (jobs.has(initial.id)) return false;
  const sourceProjectId = useRunStore.getState().sessions[threadId]?.projectId ?? useProjectStore.getState().activeProjectId;
  if (sourceProjectId) {
    try { await assertProjectSourcesSaved(sourceProjectId); }
    catch (error) { showGlobalWarning(error instanceof Error ? error.message : '请先保存源文件'); return false; }
  }
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
    epoch === currentHistoryEpoch() &&
    !controller.signal.aborted &&
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
    if (epoch === currentHistoryEpoch() && useRunStore.getState().sessions[threadId])
      upsertCommand(threadId, { ...current, status, cancellable: false });
    jobs.delete(initial.id);
  };
  retries.set(initial.id, retry);
  jobs.set(initial.id, {
    controller,
    cancel: () => {
      if (!current.cancellable) return;
      controller.abort();
      settle('canceled');
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
      controller.signal.aborted || error instanceof RequestCanceledError ? 'canceled' : 'failed',
    );
    return false;
  } finally {
    if (jobs.get(initial.id)?.controller === controller) jobs.delete(initial.id);
  }
}
