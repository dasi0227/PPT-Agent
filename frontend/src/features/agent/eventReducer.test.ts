import { describe, expect, it } from 'vitest';
import { reducePlan, reduceSSEEvent, type TimelineItem } from './eventReducer';
import type { SSEEvent } from '../../api/types';

describe('Adaptive Runtime event reducer', () => {
  it('renders strategy.selected without creating a plan', () => {
    const event: SSEEvent = {
      event: 'strategy.selected',
      data: { strategy: 'direct_action', reason: 'single field', risk: 'low', complexity: 'low' },
    };
    const items = reduceSSEEvent([], event);
    expect(items[0]).toMatchObject({ type: 'strategy_status', strategy: 'direct_action' });
    expect(reducePlan(null, event)).toBeNull();
  });

  it('merges tool.called and tool.completed', () => {
    let items: TimelineItem[] = reduceSSEEvent([], {
      event: 'tool.called', data: { call_id: 'c1', tool: 'write_staged_slide_blueprint', args: { slide_id: 's1' } },
    });
    items = reduceSSEEvent(items, {
      event: 'tool.completed', data: { call_id: 'c1', ok: true, summary: 'staged' },
    });
    expect(items).toHaveLength(1);
    expect(items[0]).toMatchObject({ type: 'tool_call', status: 'success', observation: 'staged' });
  });

  it('promotes staged artifact to committed without duplicating it', () => {
    const artifact = { kind: 'presentation_slide', id: 's1', path: 'slides/s1/index.html' };
    let items = reduceSSEEvent([], { event: 'artifact.staged', data: { artifact } });
    items = reduceSSEEvent(items, { event: 'artifact.committed', data: { artifact } });
    expect(items).toHaveLength(1);
    expect(items[0]).toMatchObject({ type: 'artifact', delivery: 'final', artifact_id: 's1' });
  });

  it('renders verification and terminal outcomes', () => {
    let items = reduceSSEEvent([], {
      event: 'verification.completed',
      data: { verifier: 'blueprint', result: { passed: true, issues: [] } },
    });
    items = reduceSSEEvent(items, {
      event: 'run.completed',
      data: { outcome: { status: 'completed', strategy: 'direct_action', summary: 'done' } },
    });
    expect(items.map((item) => item.type)).toEqual(['verification_status', 'final_result']);
  });

  it('does not expose any chain-of-thought event type', () => {
    const itemTypes: TimelineItem['type'][] = [
      'markdown', 'tool_call', 'artifact', 'final_result', 'needs_input', 'error',
      'user_turn', 'context_status', 'strategy_status', 'verification_status',
    ];
    expect(itemTypes).not.toContain('thought');
  });
});

describe('Adaptive Runtime plan reducer', () => {
  const planCreated: SSEEvent = {
    event: 'plan.created',
    data: {
      plan: {
        id: 'plan-1', goal: 'Rebuild slide',
        steps: [
          { id: 'analyze', title: 'Analyze', status: 'pending' },
          { id: 'render', title: 'Render', status: 'pending' },
        ],
      },
    },
  };

  it('keeps Respond and DirectAction planless', () => {
    expect(reducePlan(null, { event: 'run.completed', data: { outcome: { strategy: 'respond' } } })).toBeNull();
    expect(reducePlan(null, { event: 'step.completed', data: { step_id: 'direct-action' } })).toBeNull();
  });

  it('creates and updates Compact/Full plans from canonical events', () => {
    const plan = reducePlan(null, planCreated);
    expect(plan?.id).toBe('plan-1');
    expect(plan?.steps).toHaveLength(2);
    const running = reducePlan(plan, { event: 'step.started', data: { step_id: 'render' } });
    expect(running?.steps[1].status).toBe('in_progress');
    const completed = reducePlan(running, {
      event: 'step.completed', data: { step_id: 'render', summary: 'rendered' },
    });
    expect(completed?.steps[1]).toMatchObject({ status: 'completed', detail: 'rendered' });
  });
});
