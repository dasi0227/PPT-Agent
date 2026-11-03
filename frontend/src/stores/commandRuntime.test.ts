import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { currentHistoryEpoch, invalidateHistoryRequests } from '../api/client';
import { performCommand, dropThreadCommands } from './commandRuntime';
import { useSessionStore } from './sessionStore';
import { sessionFixture } from '../test/sessionFixtures';

beforeEach(() => useSessionStore.setState({ sessions: { t1: sessionFixture() } }));
afterEach(() => { dropThreadCommands(['t1']); vi.restoreAllMocks(); });

describe('command actions', () => {
  it('does not create timeline authority from an HTTP result', async () => {
    const complete = vi.fn();
    await expect(performCommand('t1', 'cmd1', async () => 'result', complete, () => {})).resolves.toBe(true);
    expect(complete).toHaveBeenCalledWith('result');
    expect(useSessionStore.getState().sessions.t1.itemsById).toEqual({});
    expect(useSessionStore.getState().sessions.t1.commandsById).toEqual({});
  });
  it('discards late caller effects after a history switch', async () => {
    let finish!: (result: string) => void;
    const complete = vi.fn(), epoch = currentHistoryEpoch();
    const pending = performCommand('t1', 'cmd1', () => new Promise<string>(resolve => { finish = resolve; }), complete, () => {});
    invalidateHistoryRequests();
    expect(currentHistoryEpoch()).not.toBe(epoch);
    finish('old scene');
    await expect(pending).resolves.toBe(false);
    expect(complete).not.toHaveBeenCalled();
  });
});
