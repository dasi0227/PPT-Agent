import { afterEach, describe, expect, it } from 'vitest';
import { useGitCommitStore } from '../stores/gitCommitStore';
import { useSessionStore } from '../stores/sessionStore';
import { commandFixture, sessionFixture } from '../test/sessionFixtures';

afterEach(() => useSessionStore.getState().drop(['t1']));
describe('Git command view', () => {
  it('derives progress and terminal state from the shared command entity', () => {
    const command = commandFixture({ kind: 'commit', status: 'running', phase: 1 });
    useSessionStore.setState({ sessions: { t1: sessionFixture({ commandsById: { cmd1: command } }) } });
    expect(useGitCommitStore.getState().sessions.p1).toMatchObject({ operationId: 'cmd1', status: 'running' });
    useSessionStore.setState({ sessions: { t1: sessionFixture({ commandsById: {
      cmd1: { ...command, entity_revision: '2', status: 'canceled' },
    } }) } });
    expect(useGitCommitStore.getState().sessions.p1.status).toBe('canceled');
  });
});
