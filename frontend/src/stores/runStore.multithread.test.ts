import { beforeEach, describe, expect, it, vi } from 'vitest';

const connections: Array<{ runId: string; onMessage: (event: any) => void; closed: boolean }> = [];
vi.mock('../api/sse', () => ({
  subscribeRunEvents: (runId: string, options: any) => {
    const connection = { runId, onMessage: options.onMessage, closed: false };
    connections.push(connection);
    return () => { connection.closed = true; };
  },
}));
vi.mock('../api/runs', () => ({
  runsApi: {
    create: async (threadId: string, payload: any) => ({
      id: `run_${threadId}`, thread_id: threadId, project_id: 'p1', status: 'running',
      target: payload.target, interaction: payload.interaction, events_url: '',
    }),
    submitInput: async () => ({}),
    cancel: async () => ({}),
    get: async () => { throw new Error('unused'); },
  },
}));

import { IDLE_SESSION, useRunStore } from './runStore';

const request = (instruction: string) => ({
  target: { artifact: 'presentation' as const, level: 'deck' as const },
  interaction: { intent: 'execute' as const },
  instruction,
});
const base = (runId: string) => ({
  schema_version: 2, run_id: runId, occurred_at: '2026-08-02T10:30:00Z',
});

describe('runStore multithread isolation', () => {
  beforeEach(() => {
    connections.length = 0;
    sessionStorage.clear();
    useRunStore.setState({ sessions: {} });
  });

  it('returns stable idle state for a missing task', () => {
    expect(useRunStore.getState().getSession('missing')).toBe(IDLE_SESSION);
  });

  it('keeps timeline, progress, terminal state, and streams isolated', async () => {
    await useRunStore.getState().createRun('tA', request('A'));
    await useRunStore.getState().createRun('tB', request('B'));
    const [connectionA, connectionB] = connections;
    connectionA.onMessage({
      id: '1', event: 'message.reasoning',
      data: { ...base('run_tA'), message_id: 'm1', text: '处理 A' },
    });
    connectionB.onMessage({
      id: '1', event: 'run.progress',
      data: { ...base('run_tB'), stage: 'writing', text: '处理 B' },
    });
    expect(useRunStore.getState().sessions.tA.timelineItems.map((item) => item.type)).toEqual(['user_turn', 'reasoning']);
    expect(useRunStore.getState().sessions.tB.timelineItems.map((item) => item.type)).toEqual(['user_turn']);
    expect(useRunStore.getState().sessions.tB.progress?.text).toBe('处理 B');

    connectionA.onMessage({
      id: '2', event: 'run.finished',
      data: { ...base('run_tA'), status: 'canceled', duration_ms: 10 },
    });
    expect(connectionA.closed).toBe(true);
    expect(connectionB.closed).toBe(false);
    expect(useRunStore.getState().sessions.tA.status).toBe('canceled');
    expect(useRunStore.getState().sessions.tB.status).toBe('running');
  });

  it('rekeys and drops a complete session without affecting siblings', async () => {
    await useRunStore.getState().createRun('draft', request('A'));
    await useRunStore.getState().createRun('other', request('B'));
    const original = useRunStore.getState().sessions.draft;
    useRunStore.getState().rekeySession('draft', 'real');
    expect(useRunStore.getState().sessions.real).toBe(original);
    expect(useRunStore.getState().sessions.draft).toBeUndefined();
    useRunStore.getState().dropSessions(['real']);
    expect(useRunStore.getState().sessions.real).toBeUndefined();
    expect(useRunStore.getState().sessions.other).toBeDefined();
  });
});
