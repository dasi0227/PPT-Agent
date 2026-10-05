import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { subscribeGitCommitExecution, type GitCommitExecution } from './gitCommits';
import type { ThreadEvent } from './threadJournal';

const mocks = vi.hoisted(() => ({ get: vi.fn(), create: vi.fn(), cancel: vi.fn(), subscribe: vi.fn(), close: vi.fn() }));
vi.mock('./commands', () => ({ commandsApi: { get: mocks.get, create: mocks.create }, cancelPersistedCommand: mocks.cancel }));
vi.mock('./threadJournal', () => ({ subscribeThreadEvents: mocks.subscribe }));

const base: GitCommitExecution = {
  command_id: 'cmd_1', attempt_id: 'attempt_2', attempt_no: 2,
  project_id: 'p1', thread_id: 't1', kind: 'commit', source: 'user',
  status: 'accepted', phase: 0, input: {}, created_at: 100, updated_at: 100,
};

function event(command: GitCommitExecution): ThreadEvent {
  return {
    id: 'event', seq: 1, ts: command.updated_at, type: `command.${command.status}`,
    command_id: command.command_id, attempt_id: command.attempt_id, turn: 'event',
    data: command as unknown as Record<string, unknown>,
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.get.mockResolvedValue(base);
  mocks.subscribe.mockReturnValue(mocks.close);
});

afterEach(() => vi.useRealTimers());

describe('Git commit command subscription', () => {
  test('delivers canonical command progress and completion', async () => {
    const onUpdate = vi.fn();
    const stop = subscribeGitCommitExecution('cmd_1', { onUpdate });
    await vi.waitFor(() => expect(mocks.subscribe).toHaveBeenCalledOnce());
    const listener = mocks.subscribe.mock.calls[0][1];
    listener.event(event({ ...base, status: 'running', phase: 1, updated_at: 200 }));
    listener.event(event({ ...base, status: 'completed', updated_at: 300, result: { empty: true } }));
    expect(onUpdate.mock.calls.map(([command]) => command.status)).toEqual(['accepted', 'running', 'completed']);
    expect(onUpdate.mock.calls[1][0]).toMatchObject({ command_id: 'cmd_1', phase: 1 });
    expect(onUpdate.mock.calls[2][0].result).toEqual({ empty: true });
    stop();
    expect(mocks.close).toHaveBeenCalledOnce();
  });

  test('ignores stale attempts, unrelated commands and updates after terminal', async () => {
    const onUpdate = vi.fn();
    const stop = subscribeGitCommitExecution('cmd_1', { onUpdate });
    await vi.waitFor(() => expect(mocks.subscribe).toHaveBeenCalledOnce());
    const listener = mocks.subscribe.mock.calls[0][1];
    listener.event(event({ ...base, attempt_no: 1, updated_at: 300 }));
    listener.event(event({ ...base, command_id: 'cmd_other', updated_at: 300 }));
    listener.event(event({ ...base, updated_at: 50 }));
    listener.event(event({ ...base, status: 'canceled', updated_at: 200 }));
    listener.event(event({ ...base, status: 'running', updated_at: 300 }));
    expect(onUpdate.mock.calls.map(([command]) => command.status)).toEqual(['accepted', 'canceled']);
    stop();
  });
});

describe('Git commit store uses command executions', () => {
  test('applies completed command results to the timeline', async () => {
    vi.useFakeTimers();
    vi.setSystemTime(1000);
    const { useGitCommitStore } = await import('../stores/gitCommitStore');
    const { useRunStore } = await import('../stores/runStore');
    useGitCommitStore.setState({ sessions: {} });
    useRunStore.setState({ sessions: {} });
    mocks.create.mockResolvedValue(base);
    expect(await useGitCommitStore.getState().start('p1', 't1')).toBe(true);
    await vi.advanceTimersByTimeAsync(0);
    const listener = mocks.subscribe.mock.calls[0][1];
    listener.event(event({ ...base, status: 'completed', updated_at: 300, result: {
      title: 'fix: update page', items: ['Update content'], branch: 'main', hash: 'abcd123',
      files_changed: 1, insertions: 2, deletions: 1, committed_at: '2026-10-06T00:00:00Z',
    } }));
    await vi.advanceTimersByTimeAsync(450);
    expect(useGitCommitStore.getState().getSession('p1').status).toBe('completed');
    expect(useRunStore.getState().sessions.t1.timelineItems).toMatchObject([
      { type: 'git_commit', operationId: 'cmd_1', status: 'completed', hash: 'abcd123' },
    ]);
    useGitCommitStore.getState().closeAll();
  });

  test('keeps the canonical canceled state without a legacy error code', async () => {
    const { useGitCommitStore } = await import('../stores/gitCommitStore');
    const { useRunStore } = await import('../stores/runStore');
    useGitCommitStore.setState({ sessions: {} });
    useRunStore.setState({ sessions: {} });
    mocks.create.mockResolvedValue(base);
    expect(await useGitCommitStore.getState().start('p1', 't1')).toBe(true);
    await vi.waitFor(() => expect(mocks.subscribe).toHaveBeenCalledOnce());
    const listener = mocks.subscribe.mock.calls[0][1];
    listener.event(event({ ...base, status: 'canceled', updated_at: 300 }));
    expect(useGitCommitStore.getState().getSession('p1').status).toBe('canceled');
    expect(useRunStore.getState().sessions.t1.timelineItems).toMatchObject([
      { type: 'git_commit', operationId: 'cmd_1', status: 'canceled' },
    ]);
    useGitCommitStore.getState().closeAll();
  });
});
