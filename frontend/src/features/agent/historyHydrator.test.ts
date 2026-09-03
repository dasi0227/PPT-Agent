import { describe, expect, it } from 'vitest';
import { hydrateRunFromHistory, type HistoryEntry } from './historyHydrator';

const base = { schema_version: 3, run_id: 'r1', occurred_at: '2026-08-02T10:30:00Z' };
const entry = (seq: number, type: string, data: Record<string, unknown>, runId = 'r1'): HistoryEntry => ({
  seq, ts: 1_754_130_600, run_id: runId, turn: type === 'user_turn' ? 'user' : 'agent', type, data,
});
const terminal = (runId = 'r1', data: Record<string, unknown> = {}) => ({
  ...base,
  run_id: runId,
  duration_ms: 10,
  affected_targets: [],
  error: null,
  trace_id: runId,
  ...data,
});

describe('history hydrator', () => {
  it('reuses public reducers for tools, plan, question, final, and terminal', () => {
    const hydrated = hydrateRunFromHistory([
      entry(1, 'user_turn', {
        text: '生成 PPT',
        scope: { artifact: 'ppt', level: 'deck' },
        mode: 'execute',
        skills: [{
          id: 'story',
          name: '演示叙事',
          description: '梳理页面叙事。',
          local_path: '/tmp/skills/story/SKILL.md',
          open_url: 'vscode://file/tmp/skills/story/SKILL.md',
        }],
        resources: [{
          kind: 'component',
          id: 'feature-card',
          name: '能力卡片',
          open_url: 'vscode://file/tmp/components/feature-card/index.html',
        }],
      }),
      entry(2, 'plan.updated', {
        ...base,
        plan: {
          plan_id: 'p1',
          revision: 1,
          title: '执行计划',
          content: '## 完整计划',
          status: 'active',
          explanation: '开始',
          steps: [{ id: 's1', title: '生成', status: 'in_progress' }],
        },
      }),
      entry(3, 'tool.started', { ...base, call_id: 'c1', tool: 'mutate_ppt', display: { label: '生成页面' } }),
      entry(4, 'tool.completed', { ...base, call_id: 'c1', tool: 'mutate_ppt', status: 'completed', display: { label: '已生成页面' } }),
      entry(5, 'question.asked', { ...base, question_id: 'q1', questions: [{ id: 'style', title: '选择风格', options: [{ id: 'tech', label: '科技' }], allow_custom: false }] }),
      entry(6, 'question.answered', { ...base, question_id: 'q1', answer: { answers: [{ question_id: 'style', selected_option_id: 'tech' }] }, display_text: '科技' }),
      entry(7, 'message.final', { ...base, message_id: 'm1', text: '已完成' }),
      entry(8, 'run.completed', terminal()),
    ]);
    expect(hydrated.plan).toMatchObject({ id: 'p1', revision: 1 });
    expect(hydrated.items.map((item) => item.type)).toEqual(['user_turn', 'tool', 'question', 'final']);
    expect(hydrated.items[0]).toMatchObject({
      type: 'user_turn',
      skills: [{ id: 'story', name: '演示叙事' }],
      components: [{ id: 'feature-card', name: '能力卡片', kind: 'component' }],
    });
    expect(hydrated.items.find((item) => item.type === 'question')).toMatchObject({ displayText: '科技' });
    expect(hydrated.session).toMatchObject({ activeRunId: 'r1', status: 'done', pendingQuestion: null });
  });

  it('never restores progress or internal trace records', () => {
    const hydrated = hydrateRunFromHistory([
      entry(1, 'run.progress', { ...base, stage: 'writing', text: '正在生成' }),
      entry(2, 'context.assembled', { profile: 'full' }),
      entry(3, 'completion.checked', { accepted: false }),
    ]);
    expect(hydrated.items).toEqual([]);
  });

  it('restores resume history and a superseded paused terminal', () => {
    const hydrated = hydrateRunFromHistory([
      entry(1, 'user_turn', {
        text: '继续生成', scope: { artifact: 'ppt', level: 'deck' }, mode: 'execute',
      }),
      entry(2, 'run.resumed', base),
      entry(3, 'run.canceled', terminal('r1', { reason: 'superseded' })),
    ]);

    expect(hydrated.items).toEqual(expect.arrayContaining([
      expect.objectContaining({ type: 'run_lifecycle', state: 'resumed' }),
      expect.objectContaining({
        type: 'terminal_notice',
        reason: 'superseded',
        message: '此前任务因服务中断而暂停，已停止执行。',
      }),
    ]));
    expect(hydrated.session.status).toBe('canceled');
  });

  it('ignores legacy command and public event shapes', () => {
    const hydrated = hydrateRunFromHistory([
      entry(1, 'user_turn', {
        text: 'legacy',
        target: { artifact: 'presentation', level: 'deck' },
        interaction: { mode: 'execute' },
      }),
      entry(2, 'message.final', {
        ...base, schema_version: 2, message_id: 'legacy-final', text: 'legacy',
      }),
    ]);
    expect(hydrated.items).toEqual([]);
    expect(hydrated.session.status).toBe('idle');
  });

  it('accepts canonical targets and rejects legacy deck targets', () => {
    const valid = hydrateRunFromHistory([
      entry(1, 'user_turn', { text: '修改整份 PPT', scope: { artifact: 'ppt', level: 'deck' }, mode: 'execute' }),
      entry(2, 'run.error', terminal('r1', {
        affected_targets: [{ type: 'deck', part: 'manifest' }],
        error: { code: 'COMMIT_FAILED', message: '保存失败', retryable: true },
      })),
    ]);
    expect(valid.items).toEqual(expect.arrayContaining([
      expect.objectContaining({ type: 'terminal_notice', message: '保存失败' }),
    ]));
    expect(valid.session.status).toBe('error');

    const legacy = hydrateRunFromHistory([
      entry(1, 'user_turn', { text: '修改整份 PPT', scope: { artifact: 'ppt', level: 'deck' }, mode: 'execute' }),
      entry(2, 'run.error', terminal('r1', {
        affected_targets: [{ type: 'deck', part: 'deck' }],
        error: { code: 'COMMIT_FAILED', message: '不应展示', retryable: true },
      })),
    ]);
    expect(legacy.items).not.toEqual(expect.arrayContaining([
      expect.objectContaining({ type: 'terminal_notice' }),
    ]));
    expect(legacy.session.status).toBe('running');
  });

  it('restores accepted and rejected steering messages from thread history', () => {
    const hydrated = hydrateRunFromHistory([
      entry(1, 'user_turn', {
        text: '开始',
        scope: { artifact: 'ppt', level: 'deck' },
        mode: 'execute',
      }),
      entry(2, 'steering', {
        client_message_id: 'msg-1', text: '改成深色', status: 'injected',
      }),
      entry(3, 'steering', {
        client_message_id: 'msg-2', text: '再加一页', status: 'rejected',
        rejection_code: 'RUN_NOT_STEERABLE',
      }),
    ]);
    expect(hydrated.items.slice(1)).toMatchObject([
      { type: 'user_turn', text: '改成深色', clientMessageId: 'msg-1', deliveryStatus: 'accepted' },
      {
        type: 'user_turn', text: '再加一页', clientMessageId: 'msg-2',
        deliveryStatus: 'rejected', rejectionCode: 'RUN_NOT_STEERABLE',
      },
    ]);
  });

  it('restores a submitted plan approval and the execute mode transition', () => {
    const plan = {
      plan_id: 'p1', revision: 1, title: '执行计划', content: '## 完整计划',
      status: 'awaiting_approval',
      steps: [{ id: 's1', title: '生成页面', status: 'pending' }],
    };
    const hydrated = hydrateRunFromHistory([
      entry(1, 'user_turn', {
        text: '先规划再执行', scope: { artifact: 'ppt', level: 'deck' }, mode: 'plan',
      }),
      entry(2, 'plan.updated', { ...base, plan }),
      entry(3, 'plan.approval_requested', { ...base, interaction_id: 'i1', plan }),
      entry(4, 'plan.approval_answered', {
        ...base, interaction_id: 'i1', plan_id: 'p1', revision: 1,
        decision: 'approve', feedback: '',
      }),
      entry(5, 'plan.updated', {
        ...base,
        plan: { ...plan, revision: 2, approved_revision: 1, status: 'active' },
      }),
      entry(6, 'run.mode_changed', { ...base, previous_mode: 'plan', mode: 'execute' }),
    ]);

    expect(hydrated.items.find((item) => item.type === 'plan_approval')).toMatchObject({
      interactionId: 'i1', answer: { decision: 'approve' },
    });
    expect(hydrated.plan).toMatchObject({ id: 'p1', revision: 2, status: 'active', approved_revision: 1 });
    expect(hydrated.session).toMatchObject({ activeRunId: 'r1', status: 'running', mode: 'execute' });
  });

  it('restores pending and answered command permissions through public validation', () => {
    const requested = entry(2, 'command.permission_requested', {
      ...base,
      interaction_id: 'cp1',
      call_id: 'c1',
      command: "sed -i '' 's/old/new/g' notes.txt",
      command_hash: 'sha256:abc',
      reason_code: 'PROJECT_FILE_EDIT',
      reason: '该命令将修改项目文件，需要你的批准。',
    });
    const pending = hydrateRunFromHistory([
      entry(1, 'user_turn', {
        text: '更新文件', scope: { artifact: 'ppt', level: 'deck' }, mode: 'execute',
      }),
      requested,
    ]);
    expect(pending.session.status).toBe('waiting');
    expect(pending.items[1]).toMatchObject({ type: 'command_permission', answer: undefined });

    const answered = hydrateRunFromHistory([
      entry(1, 'user_turn', {
        text: '更新文件', scope: { artifact: 'ppt', level: 'deck' }, mode: 'execute',
      }),
      requested,
      entry(3, 'command.permission_answered', {
        ...base,
        interaction_id: 'cp1',
        call_id: 'c1',
        command_hash: 'sha256:abc',
        decision: 'deny',
      }),
    ]);
    expect(answered.session.status).toBe('running');
    expect(answered.items[1]).toMatchObject({ type: 'command_permission', answer: 'deny' });
  });

  it('preserves append order across runs and restores the latest pending question', () => {
    const hydrated = hydrateRunFromHistory([
      entry(1, 'user_turn', {
        text: '第一轮',
        scope: { artifact: 'ppt', level: 'deck' },
        mode: 'execute',
      }, 'old'),
      entry(2, 'message.final', { ...base, run_id: 'old', message_id: 'old-final', text: '完成' }, 'old'),
      entry(3, 'run.completed', terminal('old'), 'old'),
      entry(1, 'user_turn', {
        text: '第二轮',
        scope: { artifact: 'ppt', level: 'slide', slide_id: 's2' },
        mode: 'ask',
      }, 'new'),
      entry(2, 'question.asked', {
        ...base,
        run_id: 'new',
        question_id: 'q2',
        questions: [{ id: 'direction', title: '请选择方向', options: [{ id: 'a', label: '方向 A' }], allow_custom: false }],
      }, 'new'),
    ]);
    expect(hydrated.items.filter((item) => item.type === 'user_turn').map((item) => item.text))
      .toEqual(['第一轮', '第二轮']);
    expect(hydrated.session).toMatchObject({
      activeRunId: 'new',
      status: 'waiting',
      pendingQuestion: { id: 'q2', prompt: '请选择方向' },
      scope: { level: 'slide', slide_id: 's2' },
    });
    expect(hydrated.lastEventId).toBe('2');
    expect(hydrated.plan).toBeNull();
  });

  it('restores Git commit terminal items without changing Run session state', () => {
    const hydrated = hydrateRunFromHistory([
      entry(1, 'git.commit.completed', {
        schema_version: 1,
        operation_id: 'gco_1',
        project_id: 'p1',
        thread_id: 't1',
        occurred_at: '2026-08-30T06:29:08Z',
        commit: {
          title: 'fix: align labels',
          items: ['Align labels with data points'],
          branch: 'main',
          hash: '8af42d9',
          files_changed: 3,
          insertions: 46,
          deletions: 18,
          committed_at: '2026-08-30T06:29:08Z',
        },
      }, 'gco_1'),
      entry(1, 'git.commit.failed', {
        schema_version: 1,
        operation_id: 'gco_2',
        project_id: 'p1',
        thread_id: 't1',
        occurred_at: '2026-08-30T06:31:00Z',
        error: { code: 'COMMIT_MESSAGE_INVALID', message: '提交失败', retryable: true },
      }, 'gco_2'),
    ]);
    expect(hydrated.items).toMatchObject([
      { type: 'git_commit', operationId: 'gco_1', status: 'completed', hash: '8af42d9' },
      { type: 'git_commit', operationId: 'gco_2', status: 'failed', retryable: true },
    ]);
    expect(hydrated.session.status).toBe('idle');
  });

  it('restores a briefing group as one versioned timeline item', () => {
    const hydrated = hydrateRunFromHistory([
      entry(1, 'briefing', {
        briefing_id: 'brf_1',
        project_id: 'p1',
        thread_id: 't1',
        kind: 'handoff',
        updated_at: 20,
        versions: [
          {
            briefing_id: 'brf_1', project_id: 'p1', thread_id: 't1',
            kind: 'handoff', version_no: 1, content: 'first', feedback: '', created_at: 10,
          },
          {
            briefing_id: 'brf_1', project_id: 'p1', thread_id: 't1',
            kind: 'handoff', version_no: 2, content: 'second', feedback: 'expand', created_at: 20,
          },
        ],
      }, 'brf_1'),
    ]);
    expect(hydrated.items).toHaveLength(1);
    expect(hydrated.items[0]).toMatchObject({
      type: 'briefing',
      briefingId: 'brf_1',
      kind: 'handoff',
      status: 'completed',
      versions: [{ version_no: 1 }, { version_no: 2 }],
    });
    expect(hydrated.session.status).toBe('idle');
  });
});
