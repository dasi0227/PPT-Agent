import { describe, expect, it } from 'vitest';
import { parsePublicEvent, parseSSEEvent, SSE_EVENT_NAMES } from './sse';

const base = {
  schema_version: 3,
  run_id: 'r1',
  occurred_at: '2026-08-02T10:30:00.000Z',
};
const terminal = {
  ...base,
  duration_ms: 10,
  affected_targets: [],
  error: null,
  trace_id: 'r1',
};

const payloads: Record<string, unknown> = {
  'run.started': { ...base, scope: { artifact: 'ppt', level: 'deck' }, mode: 'execute', user_input: '生成 PPT' },
  'run.progress': { ...base, stage: 'thinking', text: '正在分析' },
  'run.resumed': base,
  'run.completed': terminal,
  'run.failed': { ...terminal, error: { code: 'RUN_FAILED', message: '任务未能完成。', retryable: false } },
  'run.error': { ...terminal, error: { code: 'INTERNAL', message: '服务暂时无法完成请求。', retryable: false } },
  'run.canceled': terminal,
  'plan.updated': { ...base, plan: { plan_id: 'p1', revision: 1, title: '计划', content: '完整计划', status: 'awaiting_approval', steps: [{ id: 's1', title: '完成', status: 'pending' }] } },
  'plan.approval_requested': { ...base, interaction_id: 'i1', plan: { plan_id: 'p1', revision: 1, title: '计划', content: '完整计划', status: 'awaiting_approval', steps: [{ id: 's1', title: '完成', status: 'pending' }] } },
  'plan.approval_answered': { ...base, interaction_id: 'i1', plan_id: 'p1', revision: 1, decision: 'approve' },
  'command.permission_requested': { ...base, interaction_id: 'cp1', call_id: 'c2', command: 'cat .env', command_hash: 'sha256:abc', reason_code: 'SENSITIVE_READ', reason: '该命令将读取敏感文件。' },
  'command.permission_answered': { ...base, interaction_id: 'cp1', call_id: 'c2', command_hash: 'sha256:abc', decision: 'allow_once' },
  'run.mode_changed': { ...base, previous_mode: 'plan', mode: 'execute' },
  'message.reasoning': { ...base, message_id: 'm1', text: '先确认全局设计。' },
  'message.milestone': { ...base, message_id: 'm2', text: '全局设计已完成。', completed_step_ids: ['s1'] },
  'message.final': { ...base, message_id: 'm3', text: '已完成。' },
  'tool.started': { ...base, call_id: 'c1', tool: 'read_ppt', display: { label: '读取全局蓝图' } },
  'tool.completed': { ...base, call_id: 'c1', tool: 'read_ppt', status: 'completed', display: { label: '已读取全局设计' } },
  'question.asked': { ...base, question_id: 'q1', questions: [{ id: 'style', title: '选择风格', options: [], allow_custom: true }] },
  'question.answered': { ...base, question_id: 'q1', answer: { answers: [{ question_id: 'style', custom_text: '克制' }] }, display_text: '克制' },
};

describe('SSE parser', () => {
  it('registers and parses all 20 public events', () => {
	  expect(SSE_EVENT_NAMES).toHaveLength(20);
    for (const eventName of SSE_EVENT_NAMES) {
      expect(parseSSEEvent(eventName, JSON.stringify(payloads[eventName]), '12')).toMatchObject({
        id: '12',
        event: eventName,
      });
    }
  });

  it('validates command lifecycle projections and project file targets', () => {
    expect(parsePublicEvent('tool.started', {
      ...base,
      call_id: 'c2',
      tool: 'run_command',
      display: { label: '正在执行命令' },
      command: { text: 'git status --short' },
    })).not.toBeNull();
    expect(parsePublicEvent('tool.completed', {
      ...base,
      call_id: 'c2',
      tool: 'run_command',
      status: 'completed',
      target: { type: 'file', part: 'content', display_name: 'notes.txt' },
      display: { label: '已执行命令' },
      command: {
        text: 'git status --short',
        status: 'completed',
        exit_code: 0,
        duration_ms: 12,
        output_truncated: false,
        stdout_preview: 'M notes.txt',
      },
    })).not.toBeNull();
    expect(parsePublicEvent('tool.started', {
      ...base,
      call_id: 'c2',
      tool: 'run_command',
      display: { label: '正在执行命令' },
      command: { text: 'pwd', status: 'completed' },
    })).toBeNull();
  });

  it('ignores unknown, malformed, incomplete, and unsafe payloads', () => {
    expect(parseSSEEvent('context.assembled', JSON.stringify(base))).toBeNull();
    expect(parseSSEEvent('run.started', '{')).toBeNull();
    expect(parseSSEEvent('run.started', JSON.stringify({
      ...(payloads['run.started'] as Record<string, unknown>),
      schema_version: 2,
    }))).toBeNull();
    expect(parseSSEEvent('run.started', JSON.stringify({
      ...base,
      target: { artifact: 'presentation', level: 'deck' },
      interaction: { mode: 'execute' },
      user_input: 'legacy',
    }))).toBeNull();
    expect(parseSSEEvent('tool.started', JSON.stringify({ ...base, call_id: 'c1', tool: 'finish', display: { label: '完成' } }))).toBeNull();
    expect(parseSSEEvent('tool.completed', JSON.stringify({
      ...(payloads['tool.completed'] as Record<string, unknown>),
      observation: { html: '<section />' },
    }))).toBeNull();
    expect(parseSSEEvent('message.reasoning', JSON.stringify({
      ...(payloads['message.reasoning'] as Record<string, unknown>),
      reasoning_content: 'hidden',
    }))).toBeNull();
    expect(parseSSEEvent('run.completed', JSON.stringify({
      ...(payloads['run.completed'] as Record<string, unknown>),
      trace_id: null,
    }))).toBeNull();
  });

  it('parses tool events with local file target fields', () => {
    const started = parseSSEEvent('tool.started', JSON.stringify({
      schema_version: 3,
      run_id: 'r1',
      occurred_at: '2026-08-06T16:03:16.051323Z',
      call_id: 'mutate_ppt_4',
      tool: 'mutate_ppt',
      target: {
        type: 'deck',
        part: 'outline',
        display_name: '目录结构',
        local_path: '/Users/test/.dasi/ppt/projects/p1/outline.json',
        open_url: 'vscode://file/Users/test/.dasi/ppt/projects/p1/outline.json',
        insertions: 69,
        deletions: 9,
      },
      display: {
        label: '正在创建目录结构',
        detail: '/Users/test/.dasi/ppt/projects/p1/outline.json',
      },
    }), '1a');
    expect(started).not.toBeNull();

    const completed = parseSSEEvent('tool.completed', JSON.stringify({
      schema_version: 3,
      run_id: 'r1',
      occurred_at: '2026-08-06T16:03:17.051323Z',
      call_id: 'mutate_ppt_4',
      tool: 'mutate_ppt',
      status: 'completed',
      target: {
        type: 'deck',
        part: 'outline',
        local_path: '/Users/test/.dasi/ppt/projects/p1/outline.json',
        open_url: 'vscode://file/Users/test/.dasi/ppt/projects/p1/outline.json',
        insertions: 69,
        deletions: 9,
      },
      display: {
        label: '已创建目录结构',
        detail: '/Users/test/.dasi/ppt/projects/p1/outline.json',
      },
    }), '1');

    expect(completed).not.toBeNull();
  });

  it('parses final and completed events with local affected targets', () => {
    const finalEvent = parseSSEEvent('message.final', JSON.stringify({
      schema_version: 3,
      run_id: 'r1',
      occurred_at: '2026-08-06T16:06:31.781954Z',
      message_id: 'm1',
      text: '## 完成',
      affected_targets: [{
        type: 'deck',
        part: 'outline',
        local_path: '/Users/test/.dasi/ppt/projects/p1/outline.json',
        open_url: 'vscode://file/Users/test/.dasi/ppt/projects/p1/outline.json',
        insertions: 69,
        deletions: 9,
      }],
    }), '2');

    expect(finalEvent).not.toBeNull();

    const completedEvent = parseSSEEvent('run.completed', JSON.stringify({
      schema_version: 3,
      run_id: 'r1',
      occurred_at: '2026-08-06T16:06:31.791303Z',
      duration_ms: 1000,
      affected_targets: [{
        type: 'slide',
        slide_id: 'slide-01',
        part: 'spec',
        display_name: '页面',
        local_path: '/Users/test/.dasi/ppt/projects/p1/slides/slide-01/spec.json',
        open_url: 'vscode://file/Users/test/.dasi/ppt/projects/p1/slides/slide-01/spec.json',
        insertions: 29,
      }],
      error: null,
      trace_id: 'r1',
    }), '3');

    expect(completedEvent).not.toBeNull();
  });

  it('accepts deck snapshots through both live and structured event parsers', () => {
    const payload = {
      ...terminal,
      affected_targets: [{ type: 'deck', part: 'manifest' }],
      error: { code: 'COMMIT_FAILED', message: '修改未能安全保存，请重新发起任务。', retryable: true },
    };

    expect(parseSSEEvent('run.error', JSON.stringify(payload), '82')).toMatchObject({
      id: '82',
      event: 'run.error',
      data: { affected_targets: [{ type: 'deck', part: 'manifest' }] },
    });
    expect(parsePublicEvent('run.error', payload, '82')).toMatchObject({
      id: '82',
      event: 'run.error',
    });
  });

  it('rejects legacy deck content targets', () => {
    const event = parsePublicEvent('run.completed', {
      ...terminal,
      affected_targets: [{
        type: 'deck',
        part: 'deck',
        local_path: '/tmp/project/deck.json',
        open_url: 'vscode://file/tmp/project/deck.json',
      }],
    }, '83');

    expect(event).toBeNull();
  });

  it('rejects legacy question fields even when canonical arrays are present', () => {
    expect(parsePublicEvent('question.asked', {
      ...base,
      question_id: 'q1',
      questions: [{ id: 'style', title: '选择风格', options: [], allow_custom: true }],
      prompt: '旧问题',
    })).toBeNull();
    expect(parsePublicEvent('question.answered', {
      ...base,
      question_id: 'q1',
      answer: {
        answers: [{ question_id: 'style', custom_text: '克制' }],
        custom_text: '旧答案',
      },
      display_text: '克制',
    })).toBeNull();
  });
});
