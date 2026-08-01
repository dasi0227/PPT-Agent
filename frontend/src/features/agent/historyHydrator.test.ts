import { describe, expect, test } from 'vitest';
import { hydrateFromHistory, hydrateRunFromHistory, type HistoryEntry } from './historyHydrator';

describe('Adaptive Runtime history replay', () => {
  test.each([
    ['respond', false],
    ['direct_action', false],
    ['compact_workflow', true],
    ['full_pev', true],
  ] as const)('replays %s with the correct optional-plan shape', (strategy, hasPlan) => {
    const entries: HistoryEntry[] = [
      { seq: 1, ts: 1, run_id: 'r', turn: 'agent', type: 'strategy.selected', data: {
        strategy, reason: 'test decision', risk: 'low', complexity: 'low',
      } },
    ];
    if (hasPlan) {
      entries.push({ seq: 2, ts: 2, run_id: 'r', turn: 'agent', type: 'plan.created', data: {
        plan: { id: 'p', goal: 'Execute', steps: [{ id: 's', title: 'Step', status: 'pending' }] },
      } });
    }
    const hydrated = hydrateRunFromHistory(entries);
    expect(hydrated.strategy).toBe(strategy);
    expect(Boolean(hydrated.plan)).toBe(hasPlan);
  });

  test('replays a planless DirectAction run', () => {
    const entries: HistoryEntry[] = [
      { seq: 1, ts: 1, run_id: 'r', turn: 'user', type: 'user_turn', data: { text: 'change title' } },
      { seq: 2, ts: 2, run_id: 'r', turn: 'agent', type: 'strategy.selected', data: {
        strategy: 'direct_action', reason: 'single field', risk: 'low', complexity: 'low',
      } },
      { seq: 3, ts: 3, run_id: 'r', turn: 'agent', type: 'verification.completed', data: {
        verifier: 'blueprint', result: { passed: true, issues: [] },
      } },
      { seq: 4, ts: 4, run_id: 'r', turn: 'agent', type: 'final_result', data: {
        result: { status: 'completed', strategy: 'direct_action' },
      } },
    ];
    const hydrated = hydrateRunFromHistory(entries);
    expect(hydrated.strategy).toBe('direct_action');
    expect(hydrated.plan).toBeNull();
    expect(hydrated.items.map((item) => item.type)).toEqual([
      'user_turn', 'strategy_status', 'verification_status', 'final_result',
    ]);
  });

  test('replays Compact plan and step state', () => {
    const hydrated = hydrateRunFromHistory([
      { seq: 1, ts: 1, run_id: 'r', turn: 'agent', type: 'strategy.selected', data: {
        strategy: 'compact_workflow', reason: 'rebuild', risk: 'medium', complexity: 'medium',
      } },
      { seq: 2, ts: 2, run_id: 'r', turn: 'agent', type: 'plan.created', data: {
        plan: { id: 'p', goal: 'Rebuild', steps: [{ id: 'render', title: 'Render', status: 'pending' }] },
      } },
      { seq: 3, ts: 3, run_id: 'r', turn: 'agent', type: 'step.completed', data: {
        step_id: 'render', summary: 'done',
      } },
    ]);
    expect(hydrated.strategy).toBe('compact_workflow');
    expect(hydrated.plan?.steps[0]).toMatchObject({ status: 'completed', detail: 'done' });
  });

  test('sorts semantic history and drops unknown types', () => {
    const items = hydrateFromHistory([
      { seq: 2, ts: 2, run_id: 'r', turn: 'agent', type: 'markdown', data: { text: 'reply' } },
      { seq: 1, ts: 1, run_id: 'r', turn: 'user', type: 'user_turn', data: { text: 'hi' } },
      { seq: 3, ts: 3, run_id: 'r', turn: 'agent', type: 'unknown', data: {} },
    ]);
    expect(items.map((item) => item.type)).toEqual(['user_turn', 'markdown']);
  });
});
