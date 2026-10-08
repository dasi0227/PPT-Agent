import type { SSEEvent } from '../../api/types';

export interface RunClock {
  elapsedMs: number;
  runningSince: number | null;
}

export function runElapsed(clock: RunClock, now: number): number {
  return clock.elapsedMs + (clock.runningSince === null ? 0 : Math.max(0, now - clock.runningSince));
}

export function reduceRunClock(clock: RunClock | undefined, event: SSEEvent): RunClock | undefined {
  const timestamp = Date.parse(event.data.occurred_at);
  if (!Number.isFinite(timestamp)) return clock;
  if (event.event === 'run.started') return { elapsedMs: 0, runningSince: timestamp };
  if (!clock) return clock;
  const waiting = ['question.asked', 'plan.approval_requested', 'command.permission_requested',
    'scope.expansion_requested', 'resource.edit_approval_requested'].includes(event.event);
  if (waiting) return { elapsedMs: runElapsed(clock, timestamp), runningSince: null };
  const resumed = ['question.answered', 'plan.approval_answered', 'command.permission_answered',
    'scope.expansion_answered', 'resource.edit_approval_answered', 'run.resumed'].includes(event.event);
  if (resumed && clock.runningSince === null) return { ...clock, runningSince: timestamp };
  if (['run.completed', 'run.canceled', 'run.failed', 'run.error'].includes(event.event)) {
    return { elapsedMs: runElapsed(clock, timestamp), runningSince: null };
  }
  return clock;
}
