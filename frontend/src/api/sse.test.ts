import { describe, expect, it } from 'vitest';
import { parseSSEEvent, SSE_EVENT_NAMES } from './sse';

const base = {
  schema_version: 1,
  run_id: 'r1',
  occurred_at: '2026-08-02T10:30:00.000Z',
};

const payloads: Record<string, unknown> = {
  'run.started': { ...base, target: { artifact: 'presentation', level: 'deck' }, interaction: { intent: 'execute' }, user_input: '生成 PPT' },
  'run.progress': { ...base, stage: 'thinking', text: '正在分析' },
  'run.finished': { ...base, status: 'completed', duration_ms: 10 },
  'plan.updated': { ...base, plan: { plan_id: 'p1', revision: 1, steps: [{ id: 's1', title: '完成', status: 'pending' }] } },
  'message.reasoning': { ...base, message_id: 'm1', text: '先确认全局设计。' },
  'message.milestone': { ...base, message_id: 'm2', text: '全局设计已完成。', completed_step_ids: ['s1'] },
  'message.final': { ...base, message_id: 'm3', text: '已完成。' },
  'tool.started': { ...base, call_id: 'c1', tool: 'read_ppt', display: { label: '读取全局蓝图' } },
  'tool.completed': { ...base, call_id: 'c1', tool: 'read_ppt', status: 'completed', display: { label: '已读取全局蓝图' } },
  'question.asked': { ...base, question_id: 'q1', prompt: '选择风格', selection: 'single', options: [], allow_custom: true },
  'question.answered': { ...base, question_id: 'q1', answer: { selected_option_ids: [], custom_text: '克制' }, display_text: '克制' },
};

describe('SSE parser', () => {
  it('registers and parses exactly the 11 public events', () => {
    expect(SSE_EVENT_NAMES).toHaveLength(11);
    for (const eventName of SSE_EVENT_NAMES) {
      expect(parseSSEEvent(eventName, JSON.stringify(payloads[eventName]), '12')).toMatchObject({
        id: '12',
        event: eventName,
      });
    }
  });

  it('ignores unknown, malformed, incomplete, and unsafe payloads', () => {
    expect(parseSSEEvent('context.assembled', JSON.stringify(base))).toBeNull();
    expect(parseSSEEvent('run.started', '{')).toBeNull();
    expect(parseSSEEvent('tool.started', JSON.stringify({ ...base, call_id: 'c1', tool: 'finish', display: { label: '完成' } }))).toBeNull();
    expect(parseSSEEvent('tool.completed', JSON.stringify({
      ...(payloads['tool.completed'] as Record<string, unknown>),
      observation: { html: '<section />' },
    }))).toBeNull();
    expect(parseSSEEvent('message.reasoning', JSON.stringify({
      ...(payloads['message.reasoning'] as Record<string, unknown>),
      reasoning_content: 'hidden',
    }))).toBeNull();
  });
});
