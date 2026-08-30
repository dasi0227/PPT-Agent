import { describe, expect, it } from 'vitest';
import type { SSEEvent } from '../../api/types';
import { reducePlan, reduceSSEEvent } from './eventReducer';

const base = { schema_version: 3 as const, run_id: 'r1', occurred_at: '2026-08-02T10:30:00Z' };
const event = (name: SSEEvent['event'], data: Record<string, unknown>, id = '1') =>
  ({ id, event: name, data: { ...base, ...data } } as SSEEvent);
const terminal = (data: Record<string, unknown> = {}) => ({
  duration_ms: 20,
  affected_targets: [],
  error: null,
  trace_id: 'r1',
  ...data,
});

describe('public event reducer', () => {
  it('upserts tool completion into the started row without raw payloads', () => {
    let state = reduceSSEEvent([], event('tool.started', {
      call_id: 'c1', tool: 'mutate_ppt', plan_step_id: 'build',
      target: { type: 'slide', slide_id: 's1', part: 'html' },
      display: { label: '生成页面 s1' },
    }));
    state = reduceSSEEvent(state, event('tool.completed', {
      call_id: 'c1', tool: 'mutate_ppt', status: 'completed',
      display: { label: '已生成页面 s1', detail: '已写入暂存区' },
    }, '2'));
    expect(state).toHaveLength(1);
    expect(state[0]).toMatchObject({
      type: 'tool', callId: 'c1', status: 'completed', label: '已生成页面 s1',
    });
    expect(JSON.stringify(state[0])).not.toContain('args');
    expect(JSON.stringify(state[0])).not.toContain('observation');
  });

  it('upserts authoritative question answer and survives replay', () => {
    let state = reduceSSEEvent([], event('question.asked', {
      question_id: 'q1',
      questions: [{ id: 'style', title: '选择风格', options: [{ id: 'tech', label: '克制科技' }], allow_custom: true }],
    }));
    state = reduceSSEEvent(state, event('question.answered', {
      question_id: 'q1',
      answer: { answers: [{ question_id: 'style', selected_option_id: 'tech' }] },
      display_text: '克制科技',
    }, '2'));
    expect(state).toHaveLength(1);
    expect(state[0]).toMatchObject({ type: 'question', displayText: '克制科技' });
  });

  it('tracks command permission and the safe command projection', () => {
    let state = reduceSSEEvent([], event('command.permission_requested', {
      interaction_id: 'cp1',
      call_id: 'c2',
      command: 'cat .env',
      command_hash: 'sha256:abc',
      reason_code: 'SENSITIVE_READ',
      reason: '该命令将读取敏感文件。',
    }));
    state = reduceSSEEvent(state, event('command.permission_answered', {
      interaction_id: 'cp1',
      call_id: 'c2',
      command_hash: 'sha256:abc',
      decision: 'allow_once',
    }, '2'));
    state = reduceSSEEvent(state, event('tool.started', {
      call_id: 'c2',
      tool: 'run_command',
      display: { label: '正在执行命令' },
      command: { text: 'cat .env' },
    }, '3'));
    state = reduceSSEEvent(state, event('tool.completed', {
      call_id: 'c2',
      tool: 'run_command',
      status: 'completed',
      display: { label: '已执行命令' },
      command: { text: 'cat .env', status: 'completed', exit_code: 0, stdout_preview: '[REDACTED]' },
    }, '4'));

    expect(state).toEqual([
      expect.objectContaining({
        type: 'command_permission',
        callId: 'c2',
        answer: 'allow_once',
      }),
      expect.objectContaining({
        type: 'tool',
        callId: 'c2',
        status: 'completed',
        command: expect.objectContaining({ text: 'cat .env', stdout_preview: '[REDACTED]' }),
      }),
    ]);
  });

  it('keeps progress and plan outside timeline items', () => {
    const progress = reduceSSEEvent([], event('run.progress', { stage: 'writing', text: '正在生成页面' }));
    expect(progress).toEqual([]);
    const planEvent = event('plan.updated', {
	  plan: { plan_id: 'p1', revision: 1, title: '执行', content: '完整计划', status: 'awaiting_approval', steps: [{ id: 's1', title: '生成', status: 'in_progress' }] },
    });
    expect(reduceSSEEvent([], planEvent)).toEqual([]);
    expect(reducePlan(null, planEvent)).toMatchObject({ id: 'p1', revision: 1 });
  });

  it('records a resumed run as a compact lifecycle row', () => {
    const running = reduceSSEEvent([], event('tool.started', {
      call_id: 'interrupted',
      tool: 'read_ppt',
      display: { label: '正在读取演示内容' },
    }));
    const state = reduceSSEEvent(running, event('run.resumed', {}, '2'));
    expect(state).toEqual([
      expect.objectContaining({
        type: 'tool',
        status: 'failed',
        error: expect.objectContaining({ code: 'RUN_INTERRUPTED' }),
      }),
      expect.objectContaining({
        id: 'r1:resumed',
        type: 'run_lifecycle',
        state: 'resumed',
        text: '已从中断处恢复，继续执行',
      }),
    ]);
  });

  it('ignores stale plan revisions', () => {
    const revision2 = event('plan.updated', {
	  plan: { plan_id: 'p1', revision: 2, title: '新', content: '完整计划', status: 'active', steps: [{ id: 's1', title: '生成', status: 'completed' }] },
    });
    const revision1 = event('plan.updated', {
	  plan: { plan_id: 'p1', revision: 1, title: '旧', content: '完整计划', status: 'awaiting_approval', steps: [{ id: 's1', title: '生成', status: 'pending' }] },
    });
    const latest = reducePlan(reducePlan(null, revision2), revision1);
    expect(latest).toMatchObject({ revision: 2, title: '新' });
  });

  it('shows one final message and no completed terminal card', () => {
    let state = reduceSSEEvent([], event('message.final', {
      message_id: 'm1', text: '已完成', affected_targets: [{ type: 'slide', slide_id: 's1', part: 'html' }],
    }));
    state = reduceSSEEvent(state, event('run.completed', terminal(), '2'));
    expect(state.map((item) => item.type)).toEqual(['final']);
  });

  it('creates one compact notice for failed or canceled terminals', () => {
    let state = reduceSSEEvent([], event('run.failed', terminal({
      error: { code: 'PROVIDER_UNAVAILABLE', message: '模型服务暂时不可用', retryable: true },
    })));
    state = reduceSSEEvent(state, event('run.failed', terminal({
      error: { code: 'PROVIDER_UNAVAILABLE', message: '模型服务暂时不可用', retryable: true },
    }), '2'));
    expect(state).toHaveLength(1);
    expect(state[0]).toMatchObject({
      type: 'terminal_notice',
      status: 'failed',
      error: { code: 'PROVIDER_UNAVAILABLE', retryable: true },
    });
  });

  it('keeps runtime errors distinct from failed task results', () => {
    const state = reduceSSEEvent([], event('run.error', terminal({
      affected_targets: [{ type: 'slide', slide_id: 's1', part: 'html' }],
      error: { code: 'INTERNAL', message: '服务暂时无法完成请求。', retryable: false },
    })));
    expect(state[0]).toMatchObject({
      type: 'terminal_notice',
      status: 'error',
      affectedTargets: [{ type: 'slide', slide_id: 's1', part: 'html' }],
      traceId: 'r1',
    });
  });

  it('uses paused copy when a paused run is superseded by a new request', () => {
    const running = reduceSSEEvent([], event('tool.started', {
      call_id: 'c1',
      tool: 'read_ppt',
      display: { label: '正在读取演示内容' },
    }));
    const state = reduceSSEEvent(running, event('run.canceled', terminal({ reason: 'superseded' }), '2'));
    expect(state[0]).toMatchObject({
      type: 'tool',
      status: 'failed',
      error: { code: 'RUN_INTERRUPTED' },
    });
    expect(state[1]).toMatchObject({
      type: 'terminal_notice',
      status: 'canceled',
      reason: 'superseded',
      message: '此前任务因服务中断而暂停，已停止执行。',
    });
  });
});
