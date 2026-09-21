import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { CommandTimelineItem } from '../features/agent/eventReducer';
import { cancelCommand, performCommand } from './commandRuntime';
import { useRunStore } from './runStore';

const initial: CommandTimelineItem = {
  id: 'polish:progress-test',
  type: 'command',
  kind: 'polish',
  title: '润色输入内容',
  status: 'loading',
  timestamp: 1,
};
const item = () => useRunStore.getState().sessions.t1.timelineItems[0];

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(10_000);
  useRunStore.setState({ sessions: {} });
});
afterEach(() => vi.useRealTimers());

describe('command lifecycle', () => {
  it('displays every reported phase before replacing it with the latest result', async () => {
    const pending = performCommand(
      't1',
      initial,
      async (_signal, phase) => {
        phase(0);
        phase(1);
        phase(2);
        return '最新内容';
      },
      (content) => ({ ...initial, status: 'completed', content }),
      () => {},
    );
    await vi.advanceTimersByTimeAsync(0);
    expect(item()).toMatchObject({ status: 'loading', phase: 0 });
    await vi.advanceTimersByTimeAsync(449);
    expect(item()).toMatchObject({ phase: 0 });
    await vi.advanceTimersByTimeAsync(1);
    expect(item()).toMatchObject({ status: 'loading', phase: 1 });
    await vi.advanceTimersByTimeAsync(450);
    expect(item()).toMatchObject({ status: 'loading', phase: 2 });
    await vi.advanceTimersByTimeAsync(450);
    await expect(pending).resolves.toBe(true);
    expect(item()).toMatchObject({ status: 'completed', content: '最新内容' });
    expect(useRunStore.getState().sessions.t1.timelineItems).toHaveLength(1);
  });

  it('aborts execution and ignores a late result even after retry succeeds', async () => {
    let resolveOld!: (value: string) => void;
    let oldSignal!: AbortSignal;
    const complete = vi.fn(
      (content: string): CommandTimelineItem => ({ ...initial, status: 'completed', content }),
    );
    const old = performCommand(
      't1',
      initial,
      (signal) => {
        oldSignal = signal;
        return new Promise<string>((resolve) => {
          resolveOld = resolve;
        });
      },
      complete,
      () => {},
    );
    cancelCommand(initial.id);
    expect(oldSignal.aborted).toBe(true);
    expect(item()).toMatchObject({ status: 'canceled' });
    const retry = performCommand(
      't1',
      initial,
      async () => '重试结果',
      complete,
      () => {},
    );
    await vi.runAllTimersAsync();
    await expect(retry).resolves.toBe(true);
    resolveOld('迟到的旧结果');
    await expect(old).resolves.toBe(false);
    expect(complete).toHaveBeenCalledTimes(1);
    expect(item()).toMatchObject({ content: '重试结果', status: 'completed' });
    expect(useRunStore.getState().sessions.t1.timelineItems).toHaveLength(1);
  });
});
