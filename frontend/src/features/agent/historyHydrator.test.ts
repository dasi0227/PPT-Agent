import { describe, expect, it } from 'vitest';
import { hydrateRunFromHistory, type HistoryEntry } from './historyHydrator';

const base = { schema_version: 3, run_id: 'r1', occurred_at: '2026-08-02T10:30:00Z' };
const entry = (seq: number, type: string, data: Record<string, unknown>, runId = 'r1'): HistoryEntry => ({
  seq, ts: 1_754_130_600, run_id: runId, turn: type === 'user_turn' ? 'user' : 'agent', type, data,
});

describe('history hydrator', () => {
  it('reuses public reducers for tools, plan, question, final, and terminal', () => {
    const hydrated = hydrateRunFromHistory([
      entry(1, 'user_turn', { text: '生成 PPT', scope: { artifact: 'ppt', level: 'deck' }, mode: 'execute' }),
      entry(2, 'plan.updated', { ...base, plan: { plan_id: 'p1', revision: 1, explanation: '开始', steps: [{ id: 's1', title: '生成', status: 'in_progress' }] } }),
      entry(3, 'tool.started', { ...base, call_id: 'c1', tool: 'write_ppt', display: { label: '生成页面' } }),
      entry(4, 'tool.completed', { ...base, call_id: 'c1', tool: 'write_ppt', status: 'completed', display: { label: '已生成页面' } }),
      entry(5, 'question.asked', { ...base, question_id: 'q1', prompt: '选择风格', selection: 'single', options: [{ id: 'tech', label: '科技' }], allow_custom: false }),
      entry(6, 'question.answered', { ...base, question_id: 'q1', answer: { selected_option_ids: ['tech'], custom_text: '' }, display_text: '科技' }),
      entry(7, 'message.final', { ...base, message_id: 'm1', text: '已完成' }),
      entry(8, 'run.finished', { ...base, status: 'completed', duration_ms: 10 }),
    ]);
    expect(hydrated.plan).toMatchObject({ id: 'p1', revision: 1 });
    expect(hydrated.items.map((item) => item.type)).toEqual(['user_turn', 'tool', 'question', 'final']);
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

  it('preserves append order across runs and restores the latest pending question', () => {
    const hydrated = hydrateRunFromHistory([
      entry(1, 'user_turn', {
        text: '第一轮',
        scope: { artifact: 'ppt', level: 'deck' },
        mode: 'execute',
      }, 'old'),
      entry(2, 'message.final', { ...base, run_id: 'old', message_id: 'old-final', text: '完成' }, 'old'),
      entry(3, 'run.finished', { ...base, run_id: 'old', status: 'completed', duration_ms: 10 }, 'old'),
      entry(1, 'user_turn', {
        text: '第二轮',
        scope: { artifact: 'ppt', level: 'slide', slide_id: 's2' },
        mode: 'ask',
      }, 'new'),
      entry(2, 'question.asked', {
        ...base,
        run_id: 'new',
        question_id: 'q2',
        prompt: '请选择方向',
        selection: 'single',
        options: [{ id: 'a', label: '方向 A' }],
        allow_custom: false,
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
});
