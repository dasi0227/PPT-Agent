import { beforeEach, describe, expect, it, vi } from 'vitest';

// 每个 run 一条独立的 mock EventSource，捕获 onMessage 便于手动派发事件。
const connections: Array<{ runId: string; onMessage: (e: any) => void; closed: boolean }> = [];

vi.mock('../api/sse', () => ({
  subscribeRunEvents: (runId: string, opts: any) => {
    const conn = { runId, onMessage: opts.onMessage, closed: false };
    connections.push(conn);
    return () => { conn.closed = true; };
  },
}));

vi.mock('../api/runs', () => ({
  runsApi: {
    create: async (_threadId: string, _payload: any) => ({ id: `run_${connections.length + 1}` }),
    submitInput: async () => ({}),
    cancel: async () => ({}),
  },
}));

import { useRunStore, IDLE_SESSION } from './runStore';
const request = (instruction: string, artifact: 'blueprint' | 'presentation' = 'blueprint') => ({
  target: { artifact, level: 'deck' as const },
  interaction: { intent: 'apply' as const, clarification: 'when_blocked' as const },
  instruction,
});

function lastConn() {
  return connections[connections.length - 1];
}

describe('runStore multithread sharding', () => {
  beforeEach(() => {
    connections.length = 0;
    useRunStore.setState({ sessions: {} });
  });

  it('getSession returns stable IDLE for missing thread', () => {
    expect(useRunStore.getState().getSession('nope')).toBe(IDLE_SESSION);
    expect(useRunStore.getState().getSession('nope').status).toBe('idle');
  });

  it('two threads run in parallel without cross-contamination', async () => {
    const store = useRunStore.getState();
    await store.createRun('tA', request('A'));
    await store.createRun('tB', request('B'));

    // 两条连接分别属于各自 thread
    expect(connections).toHaveLength(2);
    const connA = connections[0];
    const connB = connections[1];

    // 向 A 派发一个 thought，只应写入 A 的分片（此时 A 已含头插的 user_turn）。
    connA.onMessage({ id: '1', event: 'thought', data: { text: 'hello A' } });

    const sessions = useRunStore.getState().sessions;
    // A: user_turn + thought；B: 仅 user_turn。
    expect(sessions['tA'].timelineItems.map((i) => i.type)).toEqual(['user_turn', 'thought']);
    expect(sessions['tB'].timelineItems.map((i) => i.type)).toEqual(['user_turn']);

    // 向 B 派发 done，A 状态不受影响
    connB.onMessage({ id: '2', event: 'done', data: { result: { summary: 'B done' } } });
    const after = useRunStore.getState().sessions;
    expect(after['tB'].status).toBe('done');
    expect(after['tA'].status).toBe('running');
  });

  it('done closes only its own connection', async () => {
    const store = useRunStore.getState();
    await store.createRun('tA', request('A'));
    await store.createRun('tB', request('B'));
    const connA = connections[0];
    const connB = connections[1];

    connA.onMessage({ id: '1', event: 'done', data: { result: {} } });
    expect(connA.closed).toBe(true);
    expect(connB.closed).toBe(false);
  });

  it('closeSessions closes connections for listed threads only', async () => {
    const store = useRunStore.getState();
    await store.createRun('tA', request('A'));
    await store.createRun('tB', request('B'));
    const connA = connections[0];
    const connB = connections[1];

    useRunStore.getState().closeSessions(['tA']);
    expect(connA.closed).toBe(true);
    expect(connB.closed).toBe(false);
  });

  it('progress updates only the target session', async () => {
    const store = useRunStore.getState();
    await store.createRun('tA', request('A', 'presentation'));
    lastConn().onMessage({ id: '1', event: 'progress', data: { stage: 'page', current: 2, total: 8 } });

    expect(useRunStore.getState().sessions['tA'].progress).toEqual({ stage: 'page', current: 2, total: 8 });
    expect(useRunStore.getState().getSession('tB').progress).toBeNull();
  });

  it('rekeySession migrates full session from draft id to real id', async () => {
    const store = useRunStore.getState();
    await store.createRun('draft_x', request('A'));
    lastConn().onMessage({ id: '1', event: 'thought', data: { text: 'hi' } });
    lastConn().onMessage({ id: '2', event: 'progress', data: { stage: 'design', current: 1, total: 1 } });

    const before = useRunStore.getState().sessions['draft_x'];
    // user_turn (createRun 头插) + thought
    expect(before.timelineItems.map((i) => i.type)).toEqual(['user_turn', 'thought']);
    expect(before.status).toBe('running');

    useRunStore.getState().rekeySession('draft_x', 'real_1');

    const after = useRunStore.getState().sessions;
    expect(after['draft_x']).toBeUndefined();
    expect(after['real_1']).toBe(before); // 同一引用整块搬迁
    expect(after['real_1'].timelineItems.map((i) => i.type)).toEqual(['user_turn', 'thought']);
    expect(after['real_1'].progress).toEqual({ stage: 'design', current: 1, total: 1 });
  });

  it('rekeySession is a no-op when old id has no session', () => {
    useRunStore.getState().rekeySession('draft_missing', 'real_2');
    expect(useRunStore.getState().sessions['real_2']).toBeUndefined();
  });

  it('dropSessions closes and removes listed session keys', async () => {
    const store = useRunStore.getState();
    await store.createRun('tA', request('A'));
    await store.createRun('tB', request('B'));
    const connA = connections[0];

    useRunStore.getState().dropSessions(['tA']);
    expect(connA.closed).toBe(true);
    expect(useRunStore.getState().sessions['tA']).toBeUndefined();
    expect(useRunStore.getState().sessions['tB']).toBeDefined();
  });
});
