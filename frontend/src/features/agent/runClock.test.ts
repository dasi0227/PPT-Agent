import { describe, expect, it } from 'vitest';
import type { SSEEvent } from '../../api/types';
import { reduceRunClock, runElapsed } from './runClock';

const event = (name: SSEEvent['event'], timestamp: number) => ({
  event: name, data: { occurred_at: new Date(timestamp).toISOString() },
}) as SSEEvent;

describe('run execution clock', () => {
  it.each([
    ['question.asked', 'question.answered'],
    ['plan.approval_requested', 'plan.approval_answered'],
    ['command.permission_requested', 'command.permission_answered'],
    ['scope.expansion_requested', 'scope.expansion_answered'],
    ['resource.edit_approval_requested', 'resource.edit_approval_answered'],
  ] as const)('excludes waiting between %s and %s', (request, answer) => {
    let clock = reduceRunClock(undefined, event('run.started', 1000))!;
    clock = reduceRunClock(clock, event(request, 4000))!;
    expect(runElapsed(clock, 30000)).toBe(3000);
    clock = reduceRunClock(clock, event(answer, 30000))!;
    expect(runElapsed(clock, 32000)).toBe(5000);
    clock = reduceRunClock(clock, event(request, 33000))!;
    clock = reduceRunClock(clock, event(answer, 50000))!;
    expect(runElapsed(clock, 51000)).toBe(7000);
  });

  it('replays waiting history without counting time until the answer', () => {
    const clock = [event('run.started', 1000), event('question.asked', 6000),
      event('run.progress', 7000), event('question.answered', 20000)]
      .reduce((previous, next) => reduceRunClock(previous, next), undefined as ReturnType<typeof reduceRunClock>);
    expect(runElapsed(clock!, 23000)).toBe(8000);
  });
});
