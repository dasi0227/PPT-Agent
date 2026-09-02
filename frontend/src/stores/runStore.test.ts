import { beforeEach, describe, expect, test, vi } from 'vitest';

const slideLoads: string[] = [];
vi.mock('./projectStore', () => ({
  useProjectStore: {
    getState: () => ({
      activeProjectId: 'p1',
      loadProjectContent: async (projectId: string) => { slideLoads.push(projectId); },
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
vi.mock('../api/sse', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api/sse')>();
  return {
    ...actual,
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
  };
});

let createMode: 'resolve' | 'pending' | 'reject' = 'resolve';
let resolveCreate: ((value: any) => void) | null = null;
let recoveredRun: any = null;
let steeringMode: 'resolve' | 'reject' = 'resolve';
let cancelResponse: any = { status: 'cancel_requested', run_id: 'run_1' };
let resumeResponse: any = null;
const reconciledRuns: any[] = [];
const getRequests: string[] = [];
const createRequests: any[] = [];
const steeringRequests: any[] = [];
const cancelRequests: any[] = [];
let historyEntries: any[] = [];
vi.mock('../api/runs', () => ({
  runsApi: {
    create: (threadId: string, payload: any) => {
      createRequests.push(payload);
      const run = {
        id: `run_${connections.length + 1}`,
        thread_id: threadId,
        project_id: 'p1',
        status: 'running',
        scope: payload.scope,
        mode: payload.mode,
        events_url: '',
        model: payload.model ?? 'Kimi K3',
        skills: (payload.skill_ids ?? []).map((id: string) => ({
          id,
          name: id === 'story' ? '演示叙事' : id,
          description: 'description',
          open_url: `vscode://file/tmp/skills/${id}/SKILL.md`,
        })),
        components: (payload.component_names ?? []).map((name: string) => ({
          kind: 'component',
          id: name === '能力卡片' ? 'feature-card' : name,
          name,
          open_url: 'vscode://file/tmp/components/feature-card/index.html',
        })),
      };
      if (createMode === 'pending') return new Promise((resolve) => { resolveCreate = resolve; });
      if (createMode === 'reject') return Promise.reject(new Error('offline'));
      return Promise.resolve(run);
    },
    submitInput: async () => ({}),
    cancel: async (runId: string, reason: string) => {
      cancelRequests.push({ runId, reason });
      return cancelResponse;
    },
    resume: async () => resumeResponse,
    steer: async (runId: string, payload: any) => {
      steeringRequests.push({ runId, payload });
      if (steeringMode === 'reject') throw new Error('not steerable');
      return { status: 'accepted', run_id: runId, client_message_id: payload.client_message_id };
    },
    get: async (runId: string) => {
      getRequests.push(runId);
      if (reconciledRuns.length > 0) return reconciledRuns.shift();
      if (recoveredRun instanceof Error) throw recoveredRun;
      return recoveredRun;
    },
  },
}));
vi.mock('../api/threads', () => ({
  threadsApi: { history: async () => historyEntries },
}));

import { useRunStore } from './runStore';
import { useComposerStore } from './composerStore';
import type { TimelineItem } from '../features/agent/eventReducer';

const request = (instruction: string) => ({
  scope: { artifact: 'ppt' as const, level: 'slide' as const, slide_id: 's1' },
  mode: 'execute' as const ,
  instruction,
});
const base = { schema_version: 3, run_id: 'run_1', occurred_at: '2026-08-02T10:30:00Z' };
const terminal = (data: Record<string, unknown> = {}) => ({
  ...base,
  duration_ms: 5,
  affected_targets: [],
  error: null,
  trace_id: String((data.run_id ?? base.run_id)),
  ...data,
});

function reset() {
  useRunStore.getState().dropSessions(Object.keys(useRunStore.getState().sessions));
  vi.useRealTimers();
  slideLoads.length = 0;
  connections.length = 0;
  createMode = 'resolve';
  resolveCreate = null;
  recoveredRun = null;
  steeringMode = 'resolve';
  cancelResponse = { status: 'cancel_requested', run_id: 'run_1' };
  resumeResponse = null;
  reconciledRuns.length = 0;
  getRequests.length = 0;
  createRequests.length = 0;
  steeringRequests.length = 0;
  cancelRequests.length = 0;
  historyEntries = [];
  sessionStorage.clear();
  useComposerStore.setState({ modelProfileName: null });
  useRunStore.setState({ sessions: {} });
}

function authoritativeRun(status: 'pending' | 'running' | 'waiting' | 'paused' | 'recovering' | 'done' | 'failed' | 'canceled') {
  return {
    id: 'run_1',
    thread_id: 't1',
    project_id: 'p1',
    status,
    scope: request('').scope,
    mode: request('').mode,
    events_url: '',
    model: 'Kimi K3',
  };
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
      scope: request('').scope, mode: request('').mode, events_url: '',
    });
    await pending;
  });

  test('stores selected Skill metadata on the user turn and reuses ids for retry', async () => {
    await useRunStore.getState().createRun('t1', {
      ...request('use a skill'),
      skill_ids: ['story'],
    }, 'p1');
    expect(useRunStore.getState().sessions.t1.timelineItems[0]).toMatchObject({
      type: 'user_turn',
      skills: [{ id: 'story', name: '演示叙事' }],
    });
    expect(await useRunStore.getState().retryRun('t1')).toBe(true);
    expect(createRequests[1]).toMatchObject({ skill_ids: ['story'] });
  });

  test('stores referenced components on the user turn and reuses names for retry', async () => {
    await useRunStore.getState().createRun('t1', {
      ...request('参考能力卡片'),
      component_names: ['能力卡片'],
    }, 'p1');
    expect(useRunStore.getState().sessions.t1.timelineItems[0]).toMatchObject({
      type: 'user_turn',
      components: [{ id: 'feature-card', name: '能力卡片', kind: 'component' }],
    });
    expect(await useRunStore.getState().retryRun('t1')).toBe(true);
    expect(createRequests[1]).toMatchObject({ component_names: ['能力卡片'] });
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

  test('returns to generic running feedback after the last tool finishes', async () => {
    await useRunStore.getState().createRun('t1', request('go'));
    const connection = connections[0];
    connection.onMessage({ id: '1', event: 'run.progress', data: { ...base, stage: 'reading', text: '读取页面中' } });
    connection.onMessage({
      id: '2',
      event: 'tool.started',
      data: {
        ...base,
        call_id: 'c1',
        tool: 'read_ppt',
        display: { label: '读取整份结构' },
      },
    });
    expect(useRunStore.getState().sessions.t1.progress).toMatchObject({
      stage: 'reading', text: '读取页面中',
    });
    connection.onMessage({
      id: '3',
      event: 'tool.completed',
      data: {
        ...base,
        call_id: 'c1',
        tool: 'read_ppt',
        status: 'completed',
        display: { label: '已读取整份结构' },
      },
    });
    expect(useRunStore.getState().sessions.t1.progress).toMatchObject({
      stage: 'thinking', text: '工具调用完成，推进任务中',
    });
  });

  test('returns to running feedback after a question answer is accepted', async () => {
    await useRunStore.getState().createRun('t1', request('go'));
    const connection = connections[0];
    connection.onMessage({
      id: '1', event: 'question.asked',
      data: { ...base, question_id: 'q1', questions: [{ id: 'style', title: '选择风格', options: [], allow_custom: true }] },
    });
    expect(useRunStore.getState().sessions.t1.status).toBe('waiting');
    await useRunStore.getState().answerQuestion(
      't1', 'run_1', 'q1',
      JSON.stringify({ answers: [{ question_id: 'style', custom_text: '克制' }] }),
    );
    expect(useRunStore.getState().sessions.t1).toMatchObject({
      status: 'running',
      pendingQuestion: null,
      progress: { stage: 'thinking', text: '已收到回答，推进任务中' },
    });
    expect(useRunStore.getState().sessions.t1.timelineItems)
      .toEqual(expect.arrayContaining([
        expect.objectContaining({
          type: 'question',
          answer: { answers: [{ question_id: 'style', custom_text: '克制' }] },
        }),
      ]));
    connection.onMessage({
      id: '2', event: 'question.answered',
      data: { ...base, question_id: 'q1', answer: { answers: [{ question_id: 'style', custom_text: '克制' }] }, display_text: '克制' },
    });
    expect(useRunStore.getState().sessions.t1.status).toBe('running');
    expect(useRunStore.getState().sessions.t1.pendingQuestion).toBeNull();
  });

  test('tracks active-run steering delivery without creating a second run', async () => {
    await useRunStore.getState().createRun('t1', request('go'), 'p1');
    expect(await useRunStore.getState().steerRun('t1', 'run_1', '后续页面改成深色', 'msg-stable')).toBe(true);
    expect(steeringRequests).toEqual([{
      runId: 'run_1',
      payload: {
        expected_run_id: 'run_1',
        client_message_id: 'msg-stable',
        content: '后续页面改成深色',
      },
    }]);
    expect(createRequests).toHaveLength(1);
    const acceptedItems = useRunStore.getState().sessions.t1.timelineItems;
    expect(acceptedItems[acceptedItems.length - 1]).toMatchObject({
      type: 'user_turn',
      text: '后续页面改成深色',
      clientMessageId: 'msg-stable',
      deliveryStatus: 'accepted',
    });
  });

  test('refreshes the active-run snapshot after a structured mutation', async () => {
    await useRunStore.getState().createRun('t1', request('go'), 'p1');
    const connection = connections[0];
    connection.onMessage({
      id: 'tool-1',
      event: 'tool.completed',
      data: {
        ...base,
        call_id: 'c1',
        tool: 'mutate_ppt',
        status: 'completed',
        target: { type: 'slide', slide_id: 's1', part: 'spec', display_name: '第 1 页' },
        display: { label: '已创建第 1 页设计稿' },
      },
    });
    expect(slideLoads).toEqual(['p1']);
  });

  test('returns to running feedback after plan approval is answered', async () => {
    await useRunStore.getState().createRun('t1', request('go'), 'p1');
    const connection = connections[0];
    connection.onMessage({
      id: 'approval-requested',
      event: 'plan.approval_requested',
      data: {
        ...base,
        interaction_id: 'i1',
        plan: {
          plan_id: 'plan-1', revision: 1, title: '执行计划', content: '生成页面',
          status: 'awaiting_approval', steps: [{ id: 'step-1', title: '生成页面', status: 'pending' }],
        },
      },
    });
    expect(useRunStore.getState().sessions.t1).toMatchObject({ status: 'waiting', progress: null });

    connection.onMessage({
      id: 'approval-answered',
      event: 'plan.approval_answered',
      data: {
        ...base,
        interaction_id: 'i1', plan_id: 'plan-1', revision: 1, decision: 'approve',
      },
    });
    expect(useRunStore.getState().sessions.t1).toMatchObject({
      status: 'running',
      progress: { stage: 'thinking', text: '已收到计划，推进任务中' },
    });
  });

  test('keeps rejected steering text and retry creates a new request identity', async () => {
    await useRunStore.getState().createRun('t1', { ...request('original'), model: 'Kimi K3' }, 'p1');
    const firstRequestId = createRequests[0].client_request_id;
    steeringMode = 'reject';
    expect(await useRunStore.getState().steerRun('t1', 'run_1', 'too late', 'msg-late')).toBe(false);
    const rejectedItems = useRunStore.getState().sessions.t1.timelineItems;
    expect(rejectedItems[rejectedItems.length - 1]).toMatchObject({
      type: 'user_turn',
      text: 'too late',
      deliveryStatus: 'rejected',
    });
    expect(await useRunStore.getState().retryRun('t1')).toBe(true);
    expect(createRequests).toHaveLength(2);
    expect(createRequests[1]).toMatchObject({
      instruction: 'original',
      model: 'Kimi K3',
      scope: request('').scope,
      mode: request('').mode,
    });
    expect(createRequests[1].client_request_id).not.toBe(firstRequestId);
  });

  test('retry defaults to the original model but honors a new composer selection', async () => {
    await useRunStore.getState().createRun('t1', { ...request('original'), model: 'Kimi K3' }, 'p1');
    expect(useComposerStore.getState().modelProfileName).toBe('Kimi K3');
    useComposerStore.getState().setModelProfileName('GPT-5');

    expect(await useRunStore.getState().retryRun('t1')).toBe(true);
    expect(createRequests[1]).toMatchObject({ instruction: 'original', model: 'GPT-5' });
  });

  test('enters cancel-requested state until authoritative terminal event arrives', async () => {
    await useRunStore.getState().createRun('t1', request('go'), 'p1');
    await useRunStore.getState().cancelRun('t1', 'run_1');
    expect(useRunStore.getState().sessions.t1.status).toBe('canceling');
    connections[0].onMessage({
      id: '1', event: 'run.canceled',
      data: terminal(),
    });
    expect(useRunStore.getState().sessions.t1.status).toBe('canceled');
  });

  test.each([
    ['canceled', 'canceled'],
    ['done', 'done'],
    ['failed', 'error'],
  ] as const)('adopts direct DELETE terminal status %s', async (serverStatus, expectedStatus) => {
    await useRunStore.getState().createRun('t1', request('go'), 'p1');
    cancelResponse = { status: serverStatus, run_id: 'run_1' };
    await useRunStore.getState().cancelRun('t1', 'run_1');

    expect(useRunStore.getState().sessions.t1.status).toBe(expectedStatus);
    expect(connections).toHaveLength(1);
    expect(connections[0].closed).toBe(true);
    await vi.waitFor(() => {
      const items = useRunStore.getState().sessions.t1.timelineItems;
      expect(items.some((item) => item.type === 'terminal_notice' || item.type === 'final')).toBe(true);
    });
  });

  test('reconciles cancellation after 3s, then 5s, then 10s and resumes SSE from the cursor', async () => {
    vi.useFakeTimers();
    await useRunStore.getState().createRun('t1', request('go'), 'p1');
    connections[0].onMessage({
      id: '7', event: 'run.progress',
      data: { ...base, stage: 'thinking', text: '处理中' },
    });
    reconciledRuns.push(
      authoritativeRun('running'),
      authoritativeRun('waiting'),
      authoritativeRun('canceled'),
    );
    await useRunStore.getState().cancelRun('t1', 'run_1');

    await vi.advanceTimersByTimeAsync(2_999);
    expect(getRequests).toHaveLength(0);
    await vi.advanceTimersByTimeAsync(1);
    expect(getRequests).toHaveLength(1);
    expect(useRunStore.getState().sessions.t1.status).toBe('canceling');

    await vi.advanceTimersByTimeAsync(4_999);
    expect(getRequests).toHaveLength(1);
    await vi.advanceTimersByTimeAsync(1);
    expect(getRequests).toHaveLength(2);

    await vi.advanceTimersByTimeAsync(9_999);
    expect(getRequests).toHaveLength(2);
    await vi.advanceTimersByTimeAsync(1);
    expect(getRequests).toHaveLength(3);
    expect(useRunStore.getState().sessions.t1.status).toBe('canceled');
    expect(connections).toHaveLength(1);
    expect(connections[0].closed).toBe(true);
    await vi.waitFor(() => {
      expect(useRunStore.getState().sessions.t1.timelineItems)
        .toEqual(expect.arrayContaining([expect.objectContaining({ type: 'terminal_notice', status: 'canceled' })]));
    });
  });

  test('stops cancellation reconciliation after terminal SSE, run replacement, or session cleanup', async () => {
    vi.useFakeTimers();

    await useRunStore.getState().createRun('t1', request('terminal'), 'p1');
    await useRunStore.getState().cancelRun('t1', 'run_1');
    connections[0].onMessage({
      id: '1', event: 'run.canceled',
      data: terminal(),
    });
    await vi.advanceTimersByTimeAsync(20_000);
    expect(getRequests).toHaveLength(0);

    await useRunStore.getState().createRun('t1', request('replace me'), 'p1');
    const replacedRunId = useRunStore.getState().sessions.t1.activeRunId!;
    cancelResponse = { status: 'cancel_requested', run_id: replacedRunId };
    await useRunStore.getState().cancelRun('t1', replacedRunId);
    await useRunStore.getState().createRun('t1', request('replacement'), 'p1');
    await vi.advanceTimersByTimeAsync(20_000);
    expect(getRequests).toHaveLength(0);

    const activeRunId = useRunStore.getState().sessions.t1.activeRunId!;
    cancelResponse = { status: 'cancel_requested', run_id: activeRunId };
    await useRunStore.getState().cancelRun('t1', activeRunId);
    useRunStore.getState().clearRun('t1');
    await vi.advanceTimersByTimeAsync(20_000);
    expect(getRequests).toHaveLength(0);
  });

  test('keeps canceling after the final non-terminal reconciliation without polling forever', async () => {
    vi.useFakeTimers();
    await useRunStore.getState().createRun('t1', request('go'), 'p1');
    reconciledRuns.push(
      authoritativeRun('running'),
      authoritativeRun('running'),
      authoritativeRun('running'),
    );
    await useRunStore.getState().cancelRun('t1', 'run_1');
    await vi.advanceTimersByTimeAsync(18_000);

    expect(getRequests).toHaveLength(3);
    expect(useRunStore.getState().sessions.t1.status).toBe('canceling');
    expect(vi.getTimerCount()).toBe(0);
    expect(useRunStore.getState().sessions.t1.timelineItems)
      .not.toEqual(expect.arrayContaining([expect.objectContaining({ type: 'terminal_notice' })]));
  });

  test('recovers a persisted canceling run and resumes bounded reconciliation', async () => {
    vi.useFakeTimers();
    sessionStorage.setItem('ppt-agent-active-runs-v1', JSON.stringify({
      t1: { runId: 'run_1', threadId: 't1', projectId: 'p1', lastEventId: '17', canceling: true },
    }));
    recoveredRun = authoritativeRun('running');
    await useRunStore.getState().recoverPersistedRuns();

    expect(useRunStore.getState().sessions.t1).toMatchObject({
      activeRunId: 'run_1', status: 'canceling', lastEventId: '17',
    });
    expect(connections[0]).toMatchObject({ runId: 'run_1', lastEventId: '17' });
    useRunStore.getState().clearRun('t1');
  });

  test('completed run closes its stream and refreshes the committed project once', async () => {
    await useRunStore.getState().createRun('t1', request('go'), 'p1');
    const connection = connections[0];
    connection.onMessage({ id: '1', event: 'message.final', data: { ...base, message_id: 'm1', text: '已完成' } });
    connection.onMessage({
      id: '2', event: 'run.completed',
      data: terminal({
        duration_ms: 20,
        affected_targets: [
          { type: 'slide', slide_id: 's1', part: 'spec' },
          { type: 'slide', slide_id: 's2', part: 'spec' },
        ],
      }),
    });
    expect(useRunStore.getState().sessions.t1.status).toBe('done');
    expect(connection.closed).toBe(true);
    expect(slideLoads).toEqual(['p1']);
    expect(useRunStore.getState().sessions.t1.timelineItems.filter((item) => item.type === 'final')).toHaveLength(1);
  });

  test('failed run refreshes once to discard the active overlay', async () => {
    await useRunStore.getState().createRun('t1', request('go'));
    connections[0].onMessage({
      id: '1', event: 'run.failed',
      data: terminal({ duration_ms: 10, error: { code: 'E', message: '失败', retryable: true } }),
    });
    expect(slideLoads).toEqual(['p1']);
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
      scope: request('').scope, mode: request('').mode, events_url: '',
      model: 'Kimi K3',
    };
    await useRunStore.getState().recoverPersistedRuns();
    expect(useRunStore.getState().sessions.t1).toMatchObject({
      activeRunId: 'saved', status: 'running', lastEventId: '17',
    });
    expect(connections[0]).toMatchObject({ runId: 'saved', lastEventId: '17' });
  });

  test('keeps a restarted run paused until the user explicitly resumes it', async () => {
    sessionStorage.setItem('ppt-agent-active-runs-v1', JSON.stringify({
      t1: { runId: 'run_1', threadId: 't1', projectId: 'p1', lastEventId: '17' },
    }));
    recoveredRun = authoritativeRun('paused');
    await useRunStore.getState().recoverPersistedRuns();

    expect(useRunStore.getState().sessions.t1).toMatchObject({
      activeRunId: 'run_1', status: 'paused', streamStatus: 'closed', lastEventId: '17',
    });
    expect(connections).toHaveLength(0);
    expect(sessionStorage.getItem('ppt-agent-active-runs-v1')).toContain('run_1');

    resumeResponse = authoritativeRun('recovering');
    expect(await useRunStore.getState().resumeRun('t1', 'run_1')).toBe(true);
    expect(useRunStore.getState().sessions.t1).toMatchObject({
      status: 'recovering',
      progress: null,
      timelineItems: expect.arrayContaining([
        expect.objectContaining({ type: 'run_lifecycle', state: 'resumed' }),
      ]),
    });
    expect(connections[0]).toMatchObject({ runId: 'run_1', lastEventId: '17' });
  });

  test('ends a paused run with superseded copy before a new request', async () => {
    await useRunStore.getState().createRun('t1', request('old request'), 'p1');
    useRunStore.setState((state) => ({
      sessions: {
        ...state.sessions,
        t1: { ...state.sessions.t1, status: 'paused', streamStatus: 'closed', progress: null },
      },
    }));
    cancelResponse = { status: 'canceled', run_id: 'run_1' };

    expect(await useRunStore.getState().cancelRun('t1', 'run_1', 'superseded')).toBe(true);
    expect(cancelRequests).toEqual([{ runId: 'run_1', reason: 'superseded' }]);
    expect(useRunStore.getState().sessions.t1).toMatchObject({
      activeRunId: null,
      status: 'canceled',
      timelineItems: expect.arrayContaining([
        expect.objectContaining({
          type: 'terminal_notice',
          reason: 'superseded',
          message: '此前任务因服务中断而暂停，已停止执行。',
        }),
      ]),
    });
  });

  test('closes a failed stream and restores its terminal event without reconnecting', async () => {
    await useRunStore.getState().createRun('t1', request('go'), 'p1');
    recoveredRun = authoritativeRun('failed');
    historyEntries = [
      {
        seq: 1,
        ts: 1,
        run_id: 'run_1',
        turn: 'user',
        type: 'user_turn',
        data: { text: 'go', scope: request('').scope, mode: 'execute' },
      },
      {
        seq: 2,
        ts: 2,
        run_id: 'run_1',
        turn: 'agent',
        type: 'run.error',
        data: terminal({
          affected_targets: [{ type: 'deck', part: 'manifest' }],
          error: { code: 'COMMIT_FAILED', message: '修改未能安全保存，请重新发起任务。', retryable: true },
        }),
      },
    ];

    connections[0].onError?.(new Event('error'));
    await vi.waitFor(() => {
      expect(useRunStore.getState().sessions.t1.timelineItems).toEqual(expect.arrayContaining([
        expect.objectContaining({
          type: 'terminal_notice',
          message: '修改未能安全保存，请重新发起任务。',
        }),
      ]));
    });

    expect(useRunStore.getState().sessions.t1).toMatchObject({
      status: 'error',
      streamStatus: 'closed',
      lastEventId: '2',
    });
    expect(connections).toHaveLength(1);
    expect(connections[0].closed).toBe(true);
    expect(slideLoads).toEqual(['p1']);
  });

  test('reconciles an open stream to paused after the backend restarts', async () => {
    await useRunStore.getState().createRun('t1', request('go'), 'p1');
    recoveredRun = authoritativeRun('paused');

    connections[0].onError?.(new Event('error'));
    await vi.waitFor(() => expect(useRunStore.getState().sessions.t1.status).toBe('paused'));

    expect(connections[0].closed).toBe(true);
    expect(useRunStore.getState().sessions.t1.streamStatus).toBe('closed');
    expect(sessionStorage.getItem('ppt-agent-active-runs-v1')).toContain('run_1');
  });

  test('hydrateTimeline is idempotent', () => {
    const seed: TimelineItem = {
      id: 'existing', type: 'reasoning', messageId: 'm1', text: 'seed', timestamp: 1,
    };
    useRunStore.setState({
      sessions: {
        t1: {
          activeRunId: null, status: 'idle',
          scope: request('').scope, mode: request('').mode,
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
