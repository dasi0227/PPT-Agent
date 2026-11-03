import { describe, expect, it } from 'vitest';
import { runElapsed } from './runClock';

describe('run clock display', () => {
  it('keeps persisted paused time and never subtracts time for clock skew', () => {
    expect(runElapsed({ elapsedMs: 3000, runningSince: null }, 50000)).toBe(3000);
    expect(runElapsed({ elapsedMs: 3000, runningSince: 5000 }, 4000)).toBe(3000);
    expect(runElapsed({ elapsedMs: 3000, runningSince: 5000 }, 6000)).toBe(4000);
  });
});
