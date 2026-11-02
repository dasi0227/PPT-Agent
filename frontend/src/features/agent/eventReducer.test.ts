import { describe, expect, it } from 'vitest';
import type { SSEEvent } from '../../api/types';
import { reducePlan, reduceSSEEvent } from './eventReducer';

const base = { schema_version: 6 as const, run_id: 'r1', occurred_at: '2026-08-02T10:30:00Z' };
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
  it.each(['run.failed', 'run.error', 'run.canceled', 'run.completed'] as const)('settles unresolved tools on %s without changing completed work', kind => {
    let state = reduceSSEEvent([], event('tool.started', { call_id: 'pending', tool: 'read_resource', display: { label: '正在读取' } }));
    state = reduceSSEEvent(state, event('tool.completed', { call_id: 'done', tool: 'read_resource', status: 'completed', display: { label: '已读取' } }));
    state = reduceSSEEvent(state, event(kind, terminal(kind === 'run.canceled' ? { reason: 'user_requested' } : {}), '3'));
    expect(state[0]).toMatchObject({ status: kind === 'run.canceled' ? 'canceled' : 'interrupted' });
    expect(state[1]).toMatchObject({ status: 'completed' });
    expect(reduceSSEEvent(state, event(kind, terminal(kind === 'run.canceled' ? { reason: 'user_requested' } : {}), '3'))).toEqual(state);
  });
  it('keeps an input reply separate from the task final and restores automatic stop replies once', () => {
    let items = reduceSSEEvent([], event('message.reply', { message_id: 'input_hi', input_id: 'hi', text: '你好' }));
    expect(items[0]).toMatchObject({ type: 'input_reply', text: '你好' });
    items = reduceSSEEvent(items, event('message.final', { message_id: 'task_final', text: '任务结果', affected_targets: [] }, '2'));
    items = reduceSSEEvent(items, event('run.completed', terminal(), '3'));
    expect(items[0]).not.toHaveProperty('durationMs');
    expect(items[1]).toMatchObject({ type: 'final', text: '任务结果', durationMs: 20 });
    const stopped = event('run.canceled', terminal({ reason: 'input_detection_failed', reply: { input_id: 'bad', text: '暂时无法处理本次请求，请稍后重试。当前任务已停止。' } }), '4');
    let failed = reduceSSEEvent([], stopped);
    failed = reduceSSEEvent(failed, stopped);
    expect(failed).toHaveLength(1);
    expect(failed[0]).toMatchObject({
      type: 'terminal_notice', reason: 'input_detection_failed',
      reply: { messageId: 'input_bad', text: '暂时无法处理本次请求，请稍后重试。当前任务已停止。' },
    });
  });
  it('invalidates the source render across runs without changing its terminal or restoring old scores', () => {
    const assessment = {
      assessment_id: 'preview_1', slide_id: 'sli_aaaaaa', material_hash: 'hash', rubric: 'preview-v4', status: 'completed',
      result: { scores: { content_coverage: 2, expression_clarity: 3, requirement_adherence: 2 }, judges: { mismatch: 'spec_mismatch', issue: 'layout_issue' } },
    };
    const completed = event('tool.completed', { call_id: 'render', tool: 'render_slide', status: 'completed', display: { label: '页面已渲染' }, preview_assessment: assessment });
    let state = reduceSSEEvent([], completed);
    state = reduceSSEEvent(state, event('tool.preview_updated', {
      run_id: 'r2', source_run_id: 'r1', call_id: 'render', preview: { ...assessment, status: 'stale', result: undefined },
    }, '2'));
    state = reduceSSEEvent(state, completed);
    expect(state).toHaveLength(1);
    expect(state[0]).toMatchObject({ type: 'tool', status: 'completed', previewAssessment: { status: 'stale', result: undefined } });
  });
  it('keeps resource approval on the edit tool and replaces the proposed diff after completion', () => {
    const proposed = { type: 'deck', part: 'meta', diff: { kind: 'fields', status: 'modified', filename: '.meta.json', fields: [] } };
    const final = { ...proposed, insertions: 2, diff: { ...proposed.diff, fields: [{ field: 'goal', rows: [] }] } };
    let state = reduceSSEEvent([], event('tool.started', { call_id: 'edit-1', tool: 'edit_meta', display: { label: '编辑基本信息' } }));
    state = reduceSSEEvent(state, event('resource.edit_approval_requested', {
      call_id: 'edit-1', interaction_id: 'approval-1', resource: 'meta', revision: 1, target: proposed,
    }, '2'));
    state = reduceSSEEvent(state, event('resource.edit_approval_updated', {
      call_id: 'edit-1', interaction_id: 'approval-1', resource: 'meta', revision: 2, target: final,
    }, '3'));
    expect(state).toHaveLength(1);
    expect(state[0]).toMatchObject({ type: 'tool', status: 'running', approval: { revision: 2, target: final } });
    state = reduceSSEEvent(state, event('resource.edit_approval_answered', {
      call_id: 'edit-1', interaction_id: 'approval-1', resource: 'meta', revision: 2, decision: 'approve',
    }, '4'));
    state = reduceSSEEvent(state, event('tool.completed', {
      call_id: 'edit-1', tool: 'edit_meta', status: 'completed', display: { label: '已编辑基本信息' }, changes: [final],
    }, '5'));
    expect(state).toHaveLength(1);
    expect(state[0]).toMatchObject({ type: 'tool', status: 'completed', changes: [final] });
    expect(state[0]).toMatchObject({ approval: { answer: { decision: 'approve' } } });
  });
  it('upserts a context compaction belonging to its run', () => {
		const compaction = {
		id: 'cmp_1',
		thread_id: 't1',
		project_id: 'p1',
		run_id: 'r1',
		trigger: 'auto' as const,
		title: '收敛上下文协议与前端实现',
		content: '## 目标与意图\n继续任务',
		before_tokens: 56000,
		after_tokens: 30000,
		max_tokens: 65536,
		reclaimed_tokens: 26000,
		duration_ms: 4200,
		created_at: 1,
	};
		let items = reduceSSEEvent([], event('context.compacted', {
      compaction: {
				...compaction,
      },
    }));
		items = reduceSSEEvent(items, event('context.compacted', {
			compaction: { ...compaction, content: '## 目标与意图\n已更新' },
		}, '2'));
		expect(items).toHaveLength(1);
    expect(items[0]).toMatchObject({
      type: 'context_compaction',
      compactionId: 'cmp_1',
      trigger: 'auto',
      runId: 'r1',
      commandSource: 'automatic',
			title: '收敛上下文协议与前端实现',
			content: '## 目标与意图\n已更新',
      reclaimedTokens: 26000,
    });
  });

  it('keeps automatic compaction in place through progress, completion and interruption', () => {
    const progress = (phase: number) => event('context.window.updated', { compaction: { id: 'cmp_progress', phase } });
    let state = reduceSSEEvent([], progress(0));
    const startedAt = state[0].timestamp;
    state = reduceSSEEvent(state, event('tool.completed', { call_id: 'read', tool: 'read_resource', status: 'completed', display: { label: '已读取' } }, '2'));
    state = reduceSSEEvent(state, progress(2));
    expect(state[0]).toMatchObject({ id: 'context-compaction:cmp_progress', runId: 'r1', phase: 2, timestamp: startedAt });
    const interrupted = reduceSSEEvent(state, event('run.canceled', terminal()));
    expect(interrupted[0]).toMatchObject({ status: 'canceled' });
    const completed = reduceSSEEvent(state, event('context.compacted', { compaction: {
      id: 'cmp_progress', trigger: 'auto', title: '整理上下文', content: '压缩后的正文',
      before_tokens: 100, after_tokens: 20, max_tokens: 200, reclaimed_tokens: 80, duration_ms: 10, created_at: 1,
    } }));
    expect(completed.map(item => item.type)).toEqual(['context_compaction', 'tool']);
    expect(completed[0]).toMatchObject({ runId: 'r1', timestamp: startedAt });
    expect(reduceSSEEvent(completed, progress(1))).toEqual(completed);
  });

  it('upserts tool completion into the started row without raw payloads', () => {
    const target = { type: 'slide', slide_id: 's1', part: 'html', display_name: '第 1 页', insertions: 1, deletions: 1,
      diff: { kind: 'text', status: 'modified', filename: 's1.html', hunks: [{ old_start: 1, old_count: 1, new_start: 1, new_count: 1,
        rows: [{ kind: 'removed', old_line: 1, text: 'Before' }, { kind: 'added', new_line: 1, text: 'After' }] }] } };
    let state = reduceSSEEvent([], event('tool.started', {
      call_id: 'c1', tool: 'edit_spec',
      target: { type: 'slide', slide_id: 's1', part: 'html' },
      display: { label: '生成页面 s1' },
    }));
    state = reduceSSEEvent(state, event('tool.completed', {
      call_id: 'c1', tool: 'edit_spec', status: 'completed',
      target, changes: [target],
      display: { label: '已生成页面 s1', detail: '已写入暂存区' },
    }, '2'));
    expect(state).toHaveLength(1);
    expect(state[0]).toMatchObject({
      type: 'tool', callId: 'c1', status: 'completed', label: '已生成页面 s1',
      target, changes: [target],
    });
    state = reduceSSEEvent(state, event('tool.started', {
      call_id: 'c1', tool: 'edit_spec', target: { type: 'slide', slide_id: 's1', part: 'html' }, display: { label: '旧的开始事件' },
    }, '3'));
    expect(state[0]).toMatchObject({ target, changes: [target], label: '已生成页面 s1' });
    expect(JSON.stringify(state[0])).not.toContain('args');
    expect(JSON.stringify(state[0])).not.toContain('observation');
  });

  it('upserts authoritative question answer and survives replay', () => {
    let state = reduceSSEEvent([], event('question.asked', {
      question_id: 'q1',
      questions: [{ id: 'style', question: '选择风格', reason: '决定视觉方向', options: [{ id: 'tech', label: '克制科技', description: '使用简洁的科技视觉风格' }] }],
    }));
    state = reduceSSEEvent(state, event('question.answered', {
      question_id: 'q1',
      answer: { answers: [{ question_id: 'style', selected_option_id: 'tech' }] },
    }, '2'));
    expect(state).toHaveLength(1);
    expect(state[0]).toMatchObject({ type: 'question', answer: { answers: [{ question_id: 'style', selected_option_id: 'tech' }] } });
  });

  it('tracks command permission and the safe command projection', () => {
    let state = reduceSSEEvent([], event('command.permission_requested', {
      interaction_id: 'cp1',
      call_id: 'c2',
      command: 'printf saved > notes.txt',
      command_hash: 'sha256:abc',
      stdout: 'saved', stderr: '', changes: [{ type: 'file', part: 'content', display_name: 'notes.txt', diff: { kind: 'binary', status: 'added', filename: 'notes.txt' } }],
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
      command: { text: 'printf saved > notes.txt' },
    }, '3'));
    state = reduceSSEEvent(state, event('tool.completed', {
      call_id: 'c2',
      tool: 'run_command',
      status: 'completed',
      display: { label: '已执行命令' },
      command: { text: 'printf saved > notes.txt', status: 'completed', exit_code: 0, stdout_preview: 'saved' },
    }, '4'));

    expect(state).toEqual([
      expect.objectContaining({
        type: 'tool',
        callId: 'c2',
        status: 'completed',
        commandApproval: expect.objectContaining({ answer: { decision: 'allow_once', feedback: undefined } }),
        command: expect.objectContaining({ text: 'printf saved > notes.txt', stdout_preview: 'saved' }),
      }),
    ]);
  });

  it('keeps progress and plan outside timeline items', () => {
    const progress = reduceSSEEvent([], event('run.progress', { activity: 'slide.updating' }));
    expect(progress).toEqual([]);
    const planEvent = event('plan.updated', {
	  plan: { steps: [{ title: '生成', status: 'processing' }] },
    });
    expect(reduceSSEEvent([], planEvent)).toEqual([]);
    const plan = reducePlan(null, planEvent);
    expect(plan?.eventRunId).toBe('r1');
    expect(plan?.steps).toStrictEqual([{ title: '生成', status: 'processing' }]);
  });

  it('records a resumed run as a compact lifecycle row', () => {
    const running = reduceSSEEvent([], event('tool.started', {
      call_id: 'interrupted',
      tool: 'read_resource',
      display: { label: '正在读取演示内容' },
    }));
    const state = reduceSSEEvent(running, event('run.resumed', {}, '2'));
    expect(state).toEqual([
      expect.objectContaining({
        type: 'tool',
        status: 'interrupted',
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

  it('replaces the full checklist and accepts an explicit clear', () => {
    const first = reducePlan(null, event('plan.updated', { plan: { steps: [{ title: '整理', status: 'completed' }] } }));
    const next = reducePlan(first, event('plan.updated', { plan: { steps: [{ title: '编写', status: 'pending' }] } }, '2'));
    expect(next?.steps).toEqual([{ title: '编写', status: 'pending' }]);
    expect(reducePlan(next, event('plan.updated', { plan: null }, '3'))).toBeNull();
  });

  it('shows one final message and no completed terminal card', () => {
    let state = reduceSSEEvent([], event('message.final', {
      message_id: 'm1', text: '已完成', affected_targets: [{ type: 'slide', slide_id: 's1', part: 'html' }],
    }));
    state = reduceSSEEvent(state, event('run.completed', terminal(), '2'));
    expect(state.map((item) => item.type)).toEqual(['final']);
    expect(state[0]).toMatchObject({ affectedTargets: [] });
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

  it('uses interruption copy when a service-interrupted run is superseded', () => {
    const running = reduceSSEEvent([], event('tool.started', {
      call_id: 'c1',
      tool: 'read_resource',
      display: { label: '正在读取演示内容' },
    }));
    const state = reduceSSEEvent(running, event('run.canceled', terminal({ reason: 'superseded' }), '2'));
    expect(state[0]).toMatchObject({
      type: 'tool',
      status: 'interrupted',
      error: { code: 'RUN_INTERRUPTED' },
    });
    expect(state[1]).toMatchObject({
      type: 'terminal_notice',
      status: 'canceled',
      reason: 'superseded',
      message: '此前任务因服务中断而结束。',
    });
  });
});
