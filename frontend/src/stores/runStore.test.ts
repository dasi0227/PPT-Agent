import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { runsApi } from '../api/runs';
import { RequestTimeoutError } from '../api/client';
import { useRunStore } from './runStore';
import { useSessionStore } from './sessionStore';
import { runFixture, sessionFixture, threadFixture } from '../test/sessionFixtures';

beforeEach(() => {
  const run = runFixture();
  useSessionStore.setState({ sessions: { t1: sessionFixture({
    thread: threadFixture({ active_run_id: run.id }), runsById: { [run.id]: run }, isEmpty: false,
  }) } });
});
afterEach(() => { vi.restoreAllMocks(); useRunStore.getState().dropSessions(['t1']); });

describe('Run action facade', () => {
  it('does not turn a cancellation timeout into a persisted terminal state', async () => {
    vi.spyOn(runsApi, 'cancel').mockRejectedValue(new RequestTimeoutError(15000));
    await expect(useRunStore.getState().cancelRun('t1', 'r1')).resolves.toBe(false);
    expect(useRunStore.getState().sessions.t1.status).toBe('running');
    expect(useRunStore.getState().sessions.t1.timelineItems).toEqual([]);
    expect(runsApi.cancel).toHaveBeenCalledWith('r1', 'user_requested', expect.objectContaining({ scene_revision: '1', entity_revision: '1' }));
  });

  it('reuses the original cancellation after an ambiguous response and later progress', async () => {
    const cancel = vi.spyOn(runsApi, 'cancel').mockRejectedValue(new RequestTimeoutError(15000));
    await useRunStore.getState().cancelRun('t1', 'r1');
    const first = cancel.mock.calls[0][2];
    useSessionStore.setState(state => ({ sessions: { ...state.sessions, t1: {
      ...state.sessions.t1, generation: state.sessions.t1.generation + 1, runsById: { r1: runFixture({ entity_revision: '2' }) },
    } } }));
    await useRunStore.getState().cancelRun('t1', 'r1');
    expect(cancel.mock.calls[1][2]).toEqual(first);
  });
});
