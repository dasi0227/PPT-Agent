import { describe, expect, it } from 'vitest';
import type { SSEEvent } from '../../api/types';
import { reducePlan, reduceSSEEvent } from './eventReducer';

const base = { schema_version: 3 as const, run_id: 'r1', occurred_at: '2026-08-02T10:30:00Z' };
const event = (name: SSEEvent['event'], data: Record<string, unknown>, id = '1') =>
  ({ id, event: name, data: { ...base, ...data } } as SSEEvent);

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
      question_id: 'q1', prompt: '选择风格', selection: 'single',
      options: [{ id: 'tech', label: '克制科技' }], allow_custom: true,
    }));
    state = reduceSSEEvent(state, event('question.answered', {
      question_id: 'q1',
      answer: { selected_option_ids: ['tech'], custom_text: '' },
      display_text: '克制科技',
    }, '2'));
    expect(state).toHaveLength(1);
    expect(state[0]).toMatchObject({ type: 'question', displayText: '克制科技' });
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
    state = reduceSSEEvent(state, event('run.finished', { status: 'completed', duration_ms: 20 }, '2'));
    expect(state.map((item) => item.type)).toEqual(['final']);
  });

  it('creates one compact notice for failed or canceled terminals', () => {
    let state = reduceSSEEvent([], event('run.finished', {
      status: 'failed', duration_ms: 20,
      error: { code: 'PROVIDER_UNAVAILABLE', message: '模型服务暂时不可用', retryable: true },
    }));
    state = reduceSSEEvent(state, event('run.finished', {
      status: 'failed', duration_ms: 20,
      error: { code: 'PROVIDER_UNAVAILABLE', message: '模型服务暂时不可用', retryable: true },
    }, '2'));
    expect(state).toHaveLength(1);
    expect(state[0]).toMatchObject({
      type: 'terminal_notice',
      status: 'failed',
      error: { code: 'PROVIDER_UNAVAILABLE', retryable: true },
    });
  });
});
