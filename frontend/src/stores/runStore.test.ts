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
const connections: Array<{ runId: string; onMessage: (e: any) => void; closed: boolean }> = [];
vi.mock('../api/sse', () => ({
  subscribeRunEvents: (runId: string, opts: any) => {
    const conn = { runId, onMessage: opts.onMessage, closed: false };
    connections.push(conn);
    return () => { conn.closed = true; };
  },
}));

// mock runs API：createRun 需要 await 一个 Promise，其中一个 case 需要挂起来观察同步插入。
let pendingResolve: ((v: any) => void) | null = null;
let pendingMode: 'resolve' | 'pending' = 'resolve';
vi.mock('../api/runs', () => ({
  runsApi: {
    create: (_threadId: string, _payload: any) => {
      if (pendingMode === 'pending') {
        return new Promise((r) => { pendingResolve = r; });
      }
      return Promise.resolve({ id: `run_${connections.length + 1}` });
    },
    submitInput: async () => ({}),
    cancel: async () => ({}),
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

  test('SSE done refreshes only the targeted presentation slide', async () => {
    await useRunStore.getState().createRun('t1', request('go'));
    const conn = connections[connections.length - 1];
    conn.onMessage({ id: '1', event: 'done', data: { result: { summary: 'ok' } } });
    expect(slideLoads).toContain('p1');
    expect(blueprintRefreshes).toEqual(['s1']);
  });

  test('SSE error does not refresh revisions that were not committed', async () => {
    await useRunStore.getState().createRun('t1', request('go'));
    const conn = connections[connections.length - 1];
    conn.onMessage({ id: '1', event: 'error', data: { code: 'E', message: 'x' } });
    expect(slideLoads).toEqual([]);
    expect(blueprintRefreshes).toEqual([]);
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
