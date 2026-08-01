import { beforeEach, describe, expect, test, vi } from 'vitest';

// 记录精确失效调用，用于验证成功按 target 刷新、失败不伪造提交。
const slideLoads: string[] = [];
const blueprintRefreshes: string[] = [];
vi.mock('./projectStore', () => ({
  useProjectStore: {
    getState: () => ({
      activeProjectId: 'p1',
      loadProjectSlides: async (pid: string) => { slideLoads.push(pid); },
    }),
  },
}));
vi.mock('./blueprintStore', () => ({
  useBlueprintStore: {
    getState: () => ({
      refreshSlide: async (_pid: string, sid: string) => { blueprintRefreshes.push(sid); },
      loadProject: async () => {},
    }),
  },
}));

// mock SSE：暴露 onMessage 供手动派发。
const connections: Array<{
  runId: string;
  onMessage: (e: any) => void;
  onError?: (e: Event) => void;
  onStatus?: (status: string) => void;
  lastEventId?: string;
  closed: boolean;
}> = [];
vi.mock('../api/sse', () => ({
  subscribeRunEvents: (runId: string, opts: any) => {
    const conn = {
      runId,
      onMessage: opts.onMessage,
      onError: opts.onError,
      onStatus: opts.onStatus,
      lastEventId: opts.lastEventId,
      closed: false,
    };
    connections.push(conn);
    return () => { conn.closed = true; };
  },
}));

// mock runs API：createRun 需要 await 一个 Promise，其中一个 case 需要挂起来观察同步插入。
let pendingResolve: ((v: any) => void) | null = null;
let pendingMode: 'resolve' | 'pending' | 'reject' = 'resolve';
let recoveredRun: any = null;
vi.mock('../api/runs', () => ({
  runsApi: {
    create: (_threadId: string, _payload: any) => {
      if (pendingMode === 'pending') {
        return new Promise((r) => { pendingResolve = r; });
      }
      if (pendingMode === 'reject') return Promise.reject(new Error('offline'));
      return Promise.resolve({
        id: `run_${connections.length + 1}`,
        thread_id: 't1',
        project_id: 'p1',
        status: 'running',
        target: request('').target,
        interaction: request('').interaction,
        events_url: '',
      });
    },
    submitInput: async () => ({}),
    cancel: async () => ({}),
    get: async () => {
      if (recoveredRun instanceof Error) throw recoveredRun;
      if (recoveredRun) return recoveredRun;
      throw new Error('not used');
    },
  },
}));

import { useRunStore } from './runStore';
import type { TimelineItem } from '../features/agent/eventReducer';
const request = (instruction: string) => ({
  target: { artifact: 'presentation' as const, level: 'slide' as const, slide_id: 's1' },
  interaction: { intent: 'apply' as const, clarification: 'when_blocked' as const },
  instruction,
});

function reset() {
  slideLoads.length = 0;
  blueprintRefreshes.length = 0;
  connections.length = 0;
  pendingResolve = null;
  pendingMode = 'resolve';
  recoveredRun = null;
  sessionStorage.clear();
  useRunStore.setState({ sessions: {} });
}

describe('runStore user_turn head insert + reload + hydrate', () => {
  beforeEach(reset);

  test('createRun inserts user_turn synchronously before network resolves', async () => {
    pendingMode = 'pending';
    const p = useRunStore.getState().createRun('t1', request('hello world'));

    // 同步断言：在 await 之前 timeline 已含 user_turn。
    const items = useRunStore.getState().sessions['t1']?.timelineItems ?? [];
    expect(items).toHaveLength(1);
    expect(items[0].type).toBe('user_turn');
    expect((items[0] as any).text).toBe('hello world');

    // 释放 create → 让 createRun 走完。
    pendingResolve?.({ id: 'r1' });
    await p.catch(() => {});
  });

  test('createRun preserves prior user_turn instead of clearing timelineItems', async () => {
    // 无预置 —— 只跑一次 createRun，断言不再有清空动作即 timelineItems=[user_turn]。
    await useRunStore.getState().createRun('t1', request('first'));
    const items = useRunStore.getState().sessions['t1'].timelineItems;
    expect(items.map((i) => i.type)).toEqual(['user_turn']);
  });

  test('createRun failure preserves the user instruction and adds an actionable error', async () => {
    pendingMode = 'reject';
    const created = await useRunStore.getState().createRun('t1', request('keep me'), 'p1');
    expect(created).toBe(false);
    expect(useRunStore.getState().sessions.t1.timelineItems.map((item) => item.type)).toEqual(['user_turn', 'error']);
    expect(useRunStore.getState().sessions.t1.status).toBe('error');
  });

  test('run.completed refreshes only the targeted presentation slide', async () => {
    await useRunStore.getState().createRun('t1', request('go'));
    const conn = connections[connections.length - 1];
    conn.onMessage({ id: '1', event: 'run.completed', data: { outcome: {
      summary: 'ok', target: request('').target, strategy: 'direct_action',
    } } });
    expect(slideLoads).toContain('p1');
    expect(blueprintRefreshes).toEqual(['s1']);
  });

  test('run.failed does not refresh revisions that were not committed', async () => {
    await useRunStore.getState().createRun('t1', request('go'));
    const conn = connections[connections.length - 1];
    conn.onMessage({ id: '1', event: 'run.failed', data: { outcome: { code: 'E', message: 'x' } } });
    expect(slideLoads).toEqual([]);
    expect(blueprintRefreshes).toEqual([]);
  });

  test('temporary stream errors reconnect without failing the run', async () => {
    await useRunStore.getState().createRun('t1', request('go'));
    const conn = connections[connections.length - 1];
    conn.onStatus?.('reconnecting');
    conn.onError?.(new Event('error'));
    expect(useRunStore.getState().sessions.t1.status).toBe('running');
    expect(useRunStore.getState().sessions.t1.streamStatus).toBe('reconnecting');
  });

  test('persists lastEventId and ignores duplicate events', async () => {
    await useRunStore.getState().createRun('t1', request('go'));
    const conn = connections[connections.length - 1];
    const event = { id: '42', event: 'status.summary', data: { summary: '执行中' } };
    conn.onMessage(event);
    conn.onMessage(event);
    expect(useRunStore.getState().sessions.t1.timelineItems.filter((item) => item.id === '42')).toHaveLength(1);
    expect(sessionStorage.getItem('ppt-agent-active-runs-v1')).toContain('"lastEventId":"42"');
  });

  test('recovers an active run with its saved last event ID', async () => {
    sessionStorage.setItem('ppt-agent-active-runs-v1', JSON.stringify({
      t1: { runId: 'run_saved', threadId: 't1', projectId: 'p1', lastEventId: '17' },
    }));
    recoveredRun = {
      id: 'run_saved',
      thread_id: 't1',
      project_id: 'p1',
      status: 'running',
      target: request('').target,
      interaction: request('').interaction,
      events_url: '/api/v1/runs/run_saved/events',
    };

    await useRunStore.getState().recoverPersistedRuns();

    expect(useRunStore.getState().sessions.t1).toMatchObject({
      activeRunId: 'run_saved',
      projectId: 'p1',
      status: 'running',
      lastEventId: '17',
    });
    expect(connections).toHaveLength(1);
    expect(connections[0]).toMatchObject({ runId: 'run_saved', lastEventId: '17' });
  });

  test('cleans a missing persisted run and leaves a non-blocking timeline notice', async () => {
    sessionStorage.setItem('ppt-agent-active-runs-v1', JSON.stringify({
      t1: { runId: 'missing', threadId: 't1', projectId: 'p1' },
    }));
    recoveredRun = new Error('not found');

    await useRunStore.getState().recoverPersistedRuns();

    expect(sessionStorage.getItem('ppt-agent-active-runs-v1')).toBeNull();
    expect(useRunStore.getState().sessions.t1.status).toBe('idle');
    expect(useRunStore.getState().sessions.t1.timelineItems[0]).toMatchObject({
      type: 'error',
      retryable: false,
    });
  });

  test('cancel marks the run as canceled', async () => {
    await useRunStore.getState().createRun('t1', request('go'));
    await useRunStore.getState().cancelRun('t1', useRunStore.getState().sessions.t1.activeRunId!);
    expect(useRunStore.getState().sessions.t1.status).toBe('canceled');
  });

  test('hydrateTimeline is idempotent when timelineItems non-empty', () => {
    const seed: TimelineItem = { id: 'existing', type: 'markdown', text: 'seed', timestamp: 1 };
    useRunStore.setState({
      sessions: {
        t1: {
          activeRunId: null,
          status: 'idle',
          target: request('').target,
          interaction: request('').interaction,
          timelineItems: [seed],
          pendingInput: null,
          progress: null,
          eventSourceClose: null,
          plan: null,
        },
      },
    });
    useRunStore.getState().hydrateTimeline('t1', [
      { id: 'hist_1', type: 'markdown', text: 'from history', timestamp: 2 },
    ]);
    const items = useRunStore.getState().sessions['t1'].timelineItems;
    expect(items).toHaveLength(1);
    expect(items[0].id).toBe('existing');
  });

  test('hydrateTimeline writes when session missing / empty', () => {
    useRunStore.getState().hydrateTimeline('t2', [
      { id: 'hist_1', type: 'markdown', text: 'restored', timestamp: 5 },
    ]);
    expect(useRunStore.getState().sessions['t2'].timelineItems).toHaveLength(1);
  });
});
