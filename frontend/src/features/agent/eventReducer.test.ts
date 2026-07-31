import { describe, it, expect } from 'vitest';
import { reduceSSEEvent, reducePlan, TimelineItem, UserTurnItem } from './eventReducer';
import { SSEEvent, PlanState } from '../../api/types';

describe('UserTurnItem type shape', () => {
  it('carries id/type/text/timestamp', () => {
    const item: UserTurnItem = { id: 'u1', type: 'user_turn', text: 'hello **world**', timestamp: 1 };
    expect(item.type).toBe('user_turn');
    expect(item.text).toBe('hello **world**');
  });
});

describe('eventReducer', () => {
  it('should reduce thought to ThoughtItem', () => {
    const state: TimelineItem[] = [];
    const event: SSEEvent = { event: 'thought', data: { text: 'thinking...' } };
    const nextState = reduceSSEEvent(state, event);
    expect(nextState).toHaveLength(1);
    expect(nextState[0].type).toBe('thought');
    expect((nextState[0] as any).text).toBe('thinking...');
  });

  it('should merge tool_call and tool_result', () => {
    let state: TimelineItem[] = [];
    const callEvent: SSEEvent = { event: 'tool_call', data: { call_id: 'call_1', tool: 'search', args: { q: 'test' } } };
    state = reduceSSEEvent(state, callEvent);
    
    expect(state[0].type).toBe('tool_call');
    expect((state[0] as any).status).toBe('running');

    const resultEvent: SSEEvent = { event: 'tool_result', data: { call_id: 'call_1', ok: true, observation: 'found 1 item' } };
    state = reduceSSEEvent(state, resultEvent);
    
    expect(state).toHaveLength(1);
    expect((state[0] as any).status).toBe('success');
    expect((state[0] as any).observation).toBe('found 1 item');
  });

  it('should distinguish artifact and done', () => {
    let state: TimelineItem[] = [];
    const artifactEvent: SSEEvent = { event: 'artifact', data: { artifact_type: 'slide_html', ref: 'slide1' } };
    state = reduceSSEEvent(state, artifactEvent);
    
    expect(state[0].type).toBe('artifact');

    const doneEvent: SSEEvent = { event: 'done', data: { result: { success: true } } };
    state = reduceSSEEvent(state, doneEvent);
    
    expect(state[1].type).toBe('final_result');
    expect((state[1] as any).result).toEqual({ success: true });
  });

  it('should attach artifact to preceding tool_call', () => {
    let state: TimelineItem[] = [];
    state = reduceSSEEvent(state, { event: 'tool_call', data: { call_id: 'c1', tool: 't1', args: {} } });
    state = reduceSSEEvent(state, { event: 'artifact', data: { artifact_type: 'img', ref: 'url' } });
    
    expect(state).toHaveLength(1);
    expect((state[0] as any).artifacts).toHaveLength(1);
  });

  it('design_spec artifact becomes standalone card (not merged into tool_call)', () => {
    let state: TimelineItem[] = [];
    state = reduceSSEEvent(state, { event: 'tool_call', data: { call_id: 'c1', tool: 'submit_design_spec', args: {} } });
    state = reduceSSEEvent(state, { event: 'artifact', data: { artifact_type: 'design_spec', ref: 'design/design-spec.json' } });
    // tool_call 不被并入；design_spec 独立成卡。
    expect(state).toHaveLength(2);
    expect(state[1].type).toBe('artifact');
    expect((state[1] as any).artifact_type).toBe('design_spec');
    expect((state[0] as any).artifacts).toHaveLength(0);
  });

  it('context.assembled becomes concise status without manifest content', () => {
    const state = reduceSSEEvent([], {
      event: 'context.assembled',
      data: { profile: 'presentation/slide', warnings: ['HTML downgraded'], read_only: true, manifest: { secret: 'x' } },
    });
    expect(state).toHaveLength(1);
    expect(state[0]).toMatchObject({
      type: 'context_status',
      profile: 'presentation/slide',
      warnings: ['HTML downgraded'],
      readOnly: true,
    });
    expect((state[0] as any).manifest).toBeUndefined();
  });
});

describe('reducePlan', () => {
  const planEvent: SSEEvent = {
    event: 'plan',
    data: {
      id: 'plan_r1', title: '构建 2 页',
      steps: [
        { id: 'p0', title: '第 1 页', status: 'pending' },
        { id: 'p1', title: '第 2 页', status: 'pending' },
      ],
    },
  };

  it('creates PlanState from plan event', () => {
    const plan = reducePlan(null, planEvent);
    expect(plan?.id).toBe('plan_r1');
    expect(plan?.steps).toHaveLength(2);
    expect(plan?.steps[0].status).toBe('pending');
  });

  it('replaces existing plan on new plan event (only one per run)', () => {
    const first = reducePlan(null, planEvent);
    const replaced = reducePlan(first, {
      event: 'plan',
      data: { id: 'plan_r1', title: 'v2', steps: [{ id: 'p0', title: 'x', status: 'pending' }] },
    });
    expect(replaced?.steps).toHaveLength(1);
    expect(replaced?.title).toBe('v2');
  });

  it('updates matching step status via plan.update', () => {
    const plan = reducePlan(null, planEvent)!;
    const updated = reducePlan(plan, {
      event: 'plan.update',
      data: { id: 'plan_r1', step_id: 'p1', status: 'in_progress' },
    });
    expect(updated?.steps[1].status).toBe('in_progress');
    expect(updated?.steps[0].status).toBe('pending'); // untouched
  });

  it('ignores plan.update with unknown step_id (V2-SSE-002)', () => {
    const plan = reducePlan(null, planEvent)!;
    const same = reducePlan(plan, {
      event: 'plan.update',
      data: { id: 'plan_r1', step_id: 'nope', status: 'completed' },
    });
    expect(same).toBe(plan); // returns prev unchanged
  });

  it('ignores plan.update when no plan or mismatched plan id', () => {
    expect(reducePlan(null, { event: 'plan.update', data: { id: 'x', step_id: 'p0', status: 'completed' } })).toBeNull();
    const plan: PlanState = { id: 'plan_r1', title: 't', steps: [{ id: 'p0', title: 'a', status: 'pending' }] };
    const mismatch = reducePlan(plan, { event: 'plan.update', data: { id: 'other', step_id: 'p0', status: 'completed' } });
    expect(mismatch).toBe(plan);
  });
});
