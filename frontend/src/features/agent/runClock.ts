export interface RunClock {
  elapsedMs: number;
  runningSince: number | null;
}

export function runElapsed(clock: RunClock, now: number): number {
  return clock.elapsedMs + (clock.runningSince === null ? 0 : Math.max(0, now - clock.runningSince));
}
