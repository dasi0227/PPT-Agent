import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { APIError } from './client';
import { commandsApi, runCommand, type CommandExecution } from './commands';
import { commandActive, dropThreadCommands, performCommand } from '../stores/commandRuntime';
import { useSessionStore } from '../stores/sessionStore';
import { useToastStore } from '../stores/toastStore';
import { commandFixture, receiptFixture, sessionFixture } from '../test/sessionFixtures';

beforeEach(() => {
  vi.useFakeTimers();
  useSessionStore.setState({ sessions: { t1: sessionFixture() } });
  vi.spyOn(useSessionStore.getState(), 'ensure').mockResolvedValue();
  vi.spyOn(useSessionStore.getState(), 'retain').mockReturnValue(vi.fn());
  useToastStore.getState().clearToasts();
});
afterEach(() => {
  dropThreadCommands(['t1']);
  vi.restoreAllMocks();
  vi.useRealTimers();
});

describe('active command status checks', () => {
  it('surfaces a terminal persistence failure and stops observing without resubmitting', async () => {
    const command = commandFixture({ kind: 'polish', status: 'running' }) as CommandExecution;
    const create = vi.spyOn(commandsApi, 'create').mockResolvedValue({ command, receipt: receiptFixture() });
    const message = '命令结果未能保存，请恢复存储后重启服务确认结果。';
    const get = vi.spyOn(commandsApi, 'get').mockResolvedValueOnce(command)
      .mockRejectedValueOnce(new APIError(500, 'PERSISTENCE_FAILED', message));
    const complete = vi.fn(), retry = vi.fn();
    const pending = performCommand('t1', 'local-command', signal => runCommand('t1', 'polish', {}, { signal }), complete, retry);
    await vi.advanceTimersByTimeAsync(0);
    expect(commandActive('local-command')).toBe(true);
    expect(get).toHaveBeenCalledOnce();
    await vi.advanceTimersByTimeAsync(2000);
    await expect(pending).resolves.toBe(false);
    expect(commandActive('local-command')).toBe(false);
    expect(complete).not.toHaveBeenCalled();
    expect(useToastStore.getState().toasts).toEqual([expect.objectContaining({ tone: 'error', message })]);
    await vi.advanceTimersByTimeAsync(10_000);
    expect(get).toHaveBeenCalledTimes(2);
    expect(create).toHaveBeenCalledOnce();
    expect(retry).not.toHaveBeenCalled();
    expect(vi.getTimerCount()).toBe(0);
  });

  it('aborts an in-flight status read after a committed terminal update and ignores its late body', async () => {
    const command = commandFixture({ kind: 'polish', status: 'running' }) as CommandExecution;
    vi.spyOn(commandsApi, 'create').mockResolvedValue({ command, receipt: receiptFixture() });
    let finish!: (command: CommandExecution) => void;
    let signal!: AbortSignal;
    const get = vi.spyOn(commandsApi, 'get').mockImplementation((_id, currentSignal) => {
      signal = currentSignal!;
      return new Promise<CommandExecution>(resolve => { finish = resolve; });
    });
    const onProgress = vi.fn();
    const pending = runCommand('t1', 'polish', {}, { onProgress });
    await vi.advanceTimersByTimeAsync(0);
    await vi.advanceTimersByTimeAsync(6000);
    expect(get).toHaveBeenCalledOnce();
    const committed = commandFixture({ kind: 'polish', status: 'completed', phase: 3,
      entity_revision: '2', updated_seq: '2', result: { content: 'saved result' } });
    useSessionStore.setState(state => ({ sessions: { t1: { ...state.sessions.t1, commandsById: { cmd1: committed } } } }));
    await expect(pending).resolves.toEqual({ content: 'saved result' });
    expect(signal.aborted).toBe(true);
    const progressCalls = onProgress.mock.calls.length;
    finish(command);
    await vi.advanceTimersByTimeAsync(10_000);
    expect(onProgress).toHaveBeenCalledTimes(progressCalls);
    expect(get).toHaveBeenCalledOnce();
    expect(vi.getTimerCount()).toBe(0);
  });
});
