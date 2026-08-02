import { beforeEach, describe, expect, test, vi } from 'vitest';

const slideLoads: string[] = [];
const specRefreshes: string[] = [];
vi.mock('./projectStore', () => ({
  useProjectStore: {
    getState: () => ({
      activeProjectId: 'p1',
      loadProjectSlides: async (projectId: string) => { slideLoads.push(projectId); },
    }),
  },
}));
vi.mock('./specStore', () => ({
  useSpecStore: {
    getState: () => ({
      refreshSlide: async (_projectId: string, slideId: string) => { specRefreshes.push(slideId); },
      loadProject: async () => {},
    }),
  },
}));

const connections: Array<{
  runId: string;
  onMessage: (event: any) => void;
  onError?: (event: Event) => void;
  onStatus?: (status: string) => void;
  lastEventId?: string;
  closed: boolean;
}> = [];
vi.mock('../api/sse', () => ({
  subscribeRunEvents: (runId: string, options: any) => {
    const connection = {
      runId,
      onMessage: options.onMessage,
      onError: options.onError,
      onStatus: options.onStatus,
      lastEventId: options.lastEventId,
      closed: false,
    };
    connections.push(connection);
    return () => { connection.closed = true; };
  },
}));

let createMode: 'resolve' | 'pending' | 'reject' = 'resolve';
let resolveCreate: ((value: any) => void) | null = null;
let recoveredRun: any = null;
vi.mock('../api/runs', () => ({
  runsApi: {
    create: (threadId: string, payload: any) => {
      const run = {
        id: `run_${connections.length + 1}`,
        thread_id: threadId,
        project_id: 'p1',
        status: 'running',
        target: payload.target,
        interaction: payload.interaction,
        events_url: '',
      };
      if (createMode === 'pending') return new Promise((resolve) => { resolveCreate = resolve; });
      if (createMode === 'reject') return Promise.reject(new Error('offline'));
      return Promise.resolve(run);
    },
    submitInput: async () => ({}),
    cancel: async () => ({}),
    get: async () => {
      if (recoveredRun instanceof Error) throw recoveredRun;
      return recoveredRun;
    },
  },
}));
vi.mock('../api/threads', () => ({
  threadsApi: { history: async () => [] },
}));

import { useRunStore } from './runStore';
import type { TimelineItem } from '../features/agent/eventReducer';

const request = (instruction: string) => ({
  target: { artifact: 'presentation' as const, level: 'slide' as const, slide_id: 's1' },
  interaction: { intent: 'execute' as const },
  instruction,
});
const base = { schema_version: 2, run_id: 'run_1', occurred_at: '2026-08-02T10:30:00Z' };

function reset() {
  slideLoads.length = 0;
  specRefreshes.length = 0;
  connections.length = 0;
  createMode = 'resolve';
  resolveCreate = null;
  recoveredRun = null;
  sessionStorage.clear();
  useRunStore.setState({ sessions: {} });
}

describe('runStore public event sessions', () => {
  beforeEach(reset);

  test('optimistically inserts a user turn before create resolves', async () => {
    createMode = 'pending';
    const pending = useRunStore.getState().createRun('t1', request('hello'));
    expect(useRunStore.getState().sessions.t1.timelineItems).toMatchObject([
      { type: 'user_turn', text: 'hello' },
    ]);
    resolveCreate?.({
      id: 'run_1', thread_id: 't1', project_id: 'p1', status: 'running',
      target: request('').target, interaction: request('').interaction, events_url: '',
    });
    await pending;
  });

  test('preserves the instruction and adds a compact local failure notice', async () => {
    createMode = 'reject';
    expect(await useRunStore.getState().createRun('t1', request('keep me'), 'p1')).toBe(false);
    expect(useRunStore.getState().sessions.t1.timelineItems.map((item) => item.type)).toEqual([
      'user_turn', 'terminal_notice',
    ]);
  });

  test('replaces progress instead of appending timeline items', async () => {
    await useRunStore.getState().createRun('t1', request('go'));
    const connection = connections[0];
    connection.onMessage({ id: '1', event: 'run.progress', data: { ...base, stage: 'writing', text: '正在生成页面' } });
    connection.onMessage({ id: '2', event: 'run.progress', data: { ...base, stage: 'rendering', text: '正在检查页面' } });
    expect(useRunStore.getState().sessions.t1.progress).toMatchObject({
      stage: 'rendering', text: '正在检查页面',
    });
    expect(useRunStore.getState().sessions.t1.timelineItems.map((item) => item.type)).toEqual(['user_turn']);
  });

  test('keeps question waiting until authoritative question.answered arrives', async () => {
    await useRunStore.getState().createRun('t1', request('go'));
    const connection = connections[0];
    connection.onMessage({
      id: '1', event: 'question.asked',
      data: { ...base, question_id: 'q1', prompt: '选择风格', selection: 'single', options: [], allow_custom: true },
    });
    expect(useRunStore.getState().sessions.t1.status).toBe('waiting');
    await useRunStore.getState().answerQuestion(
      't1', 'run_1', 'q1',
      JSON.stringify({ selected_option_ids: [], custom_text: '克制' }),
    );
    expect(useRunStore.getState().sessions.t1.status).toBe('waiting');
    connection.onMessage({
      id: '2', event: 'question.answered',
      data: { ...base, question_id: 'q1', answer: { selected_option_ids: [], custom_text: '克制' }, display_text: '克制' },
    });
    expect(useRunStore.getState().sessions.t1.status).toBe('running');
    expect(useRunStore.getState().sessions.t1.pendingQuestion).toBeNull();
  });

  test('completed run closes its stream and refreshes only committed target scope', async () => {
    await useRunStore.getState().createRun('t1', request('go'), 'p1');
    const connection = connections[0];
    connection.onMessage({ id: '1', event: 'message.final', data: { ...base, message_id: 'm1', text: '已完成' } });
    connection.onMessage({ id: '2', event: 'run.finished', data: { ...base, status: 'completed', duration_ms: 20 } });
    expect(useRunStore.getState().sessions.t1.status).toBe('done');
    expect(connection.closed).toBe(true);
    expect(slideLoads).toContain('p1');
    expect(specRefreshes).toEqual(['s1']);
    expect(useRunStore.getState().sessions.t1.timelineItems.filter((item) => item.type === 'final')).toHaveLength(1);
  });

  test('failed run does not refresh targets', async () => {
    await useRunStore.getState().createRun('t1', request('go'));
    connections[0].onMessage({
      id: '1', event: 'run.finished',
      data: { ...base, status: 'failed', duration_ms: 10, error: { code: 'E', message: '失败', retryable: true } },
    });
    expect(slideLoads).toEqual([]);
    expect(specRefreshes).toEqual([]);
  });

  test('deduplicates replayed event IDs and persists Last-Event-ID', async () => {
    await useRunStore.getState().createRun('t1', request('go'));
    const event = { id: '42', event: 'message.reasoning', data: { ...base, message_id: 'm1', text: '先设计全局' } };
    connections[0].onMessage(event);
    connections[0].onMessage(event);
    expect(useRunStore.getState().sessions.t1.timelineItems.filter((item) => item.type === 'reasoning')).toHaveLength(1);
    expect(sessionStorage.getItem('ppt-agent-active-runs-v1')).toContain('"lastEventId":"42"');
  });

  test('recovers active runs from the saved cursor', async () => {
    sessionStorage.setItem('ppt-agent-active-runs-v1', JSON.stringify({
      t1: { runId: 'saved', threadId: 't1', projectId: 'p1', lastEventId: '17' },
    }));
    recoveredRun = {
      id: 'saved', thread_id: 't1', project_id: 'p1', status: 'running',
      target: request('').target, interaction: request('').interaction, events_url: '',
    };
    await useRunStore.getState().recoverPersistedRuns();
    expect(useRunStore.getState().sessions.t1).toMatchObject({
      activeRunId: 'saved', status: 'running', lastEventId: '17',
    });
    expect(connections[0]).toMatchObject({ runId: 'saved', lastEventId: '17' });
  });

  test('hydrateTimeline is idempotent', () => {
    const seed: TimelineItem = {
      id: 'existing', type: 'reasoning', messageId: 'm1', text: 'seed', timestamp: 1,
    };
    useRunStore.setState({
      sessions: {
        t1: {
          activeRunId: null, status: 'idle',
          target: request('').target, interaction: request('').interaction,
          timelineItems: [seed], pendingQuestion: null, progress: null,
          eventSourceClose: null, plan: null,
        },
      },
    });
    useRunStore.getState().hydrateTimeline('t1', [{
      id: 'history', type: 'reasoning', messageId: 'm2', text: 'history', timestamp: 2,
    }]);
    expect(useRunStore.getState().sessions.t1.timelineItems).toEqual([seed]);
  });
});
