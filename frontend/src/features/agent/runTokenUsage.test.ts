import { describe, expect, it } from 'vitest';
import type { SSEEvent, TokenUsage } from '../../api/types';
import { hydrateRunFromHistory, type HistoryEntry } from './historyHydrator';
import { formatRunTokenUsage, mergeRunTokenUsage, reduceRunTokenUsage, type RunTokenUsageByRun } from './runTokenUsage';
import { reduceSSEEvent } from './eventReducer';

const event = (runId: string, callId: string, usage: TokenUsage | null, provider = 'openai'): Extract<SSEEvent, { event: 'run.usage.updated' }> => ({
  event: 'run.usage.updated',
  data: { schema_version: 6, run_id: runId, occurred_at: '2026-11-01T00:00:00Z',
    call: { call_id: callId, provider, model: 'model', duration_ms: 10, usage } },
});

describe('Run token consumption', () => {
  it('keeps accounting-only history out of task lifecycle and timeline state', () => {
    const usage = event('r1', 'only', { input_tokens: 10, output_tokens: 2, total_tokens: 12 });
    const items = [] as ReturnType<typeof reduceSSEEvent>;
    expect(reduceSSEEvent(items, usage)).toBe(items);
    const restored = hydrateRunFromHistory([{ seq: 1, ts: 1, run_id: 'r1', type: usage.event, turn: 'agent', data: { ...usage.data } }]);
    expect(restored.items).toEqual([]);
    expect(restored.session).toMatchObject({ status: 'idle', activeRunId: null, tokenUsageByRun: { r1: { total_tokens: 12 } } });
    expect(restored.session.runClock).toBeUndefined();
    expect(restored.lastEventId).toBeUndefined();
  });

  it('merges overlapping and older snapshots by call without losing live consumption', () => {
    const first = event('r1', 'first', { input_tokens: 10, output_tokens: 2, total_tokens: 12 });
    const newer = event('r1', 'newer', { input_tokens: 20, output_tokens: 3, total_tokens: 23 });
    const missing = event('r1', 'missing', null);
    const live = [first, newer].reduce(reduceRunTokenUsage, {});
    const snapshot = [first, missing].reduce(reduceRunTokenUsage, {});
    const merged = mergeRunTokenUsage(live, snapshot);
    expect(merged.r1).toMatchObject({ total_tokens: 35, unknown_calls: 1 });
    expect(Object.keys(merged.r1.calls)).toHaveLength(3);
    expect(mergeRunTokenUsage(merged, snapshot)).toBe(merged);
    expect(mergeRunTokenUsage(merged, {})).toBe(merged);
  });

  it('counts model requests including Jev once and keeps different Runs separate', () => {
    const first = event('r1', 'main', { input_tokens: 100, output_tokens: 20, total_tokens: 120 });
    let state: RunTokenUsageByRun = reduceRunTokenUsage({}, first);
    expect(reduceRunTokenUsage(state, first)).toBe(state);
    state = reduceRunTokenUsage(state, event('r1', 'jev', { input_tokens: 5, output_tokens: 5, total_tokens: 10 }, 'jev'));
    state = reduceRunTokenUsage(state, event('r2', 'other', { input_tokens: 200, output_tokens: 20, total_tokens: 220 }));
    state = reduceRunTokenUsage(state, event('r1', 'missing', null));
    expect(state.r1).toMatchObject({ input_tokens: 105, output_tokens: 25, total_tokens: 130, unknown_calls: 1 });
    expect(formatRunTokenUsage(state.r1)).toBe('130');
    expect(formatRunTokenUsage(state.r2)).toBe('220');
    expect(formatRunTokenUsage(undefined)).toBeNull();
    expect(formatRunTokenUsage(reduceRunTokenUsage({}, event('r1', 'unknown', null)).r1)).toBeNull();
    expect(formatRunTokenUsage(reduceRunTokenUsage({}, event('r1', 'zero', { input_tokens: 0, output_tokens: 0, total_tokens: 0 })).r1)).toBe('0');
  });

  it('restores consumption across termination, resumption and late older-Run events', () => {
    const records: HistoryEntry[] = [];
    const append = (value: SSEEvent) => records.push({ seq: records.length + 1, ts: records.length + 1,
      run_id: value.data.run_id, type: value.event, turn: 'agent', data: { ...value.data } });
    const start = (runId: string): SSEEvent => ({ event: 'run.started', data: {
      schema_version: 6, run_id: runId, occurred_at: '2026-11-01T00:00:00Z', mode: 'chat', user_input: 'test',
      scope: { slide_ids: [], source: { kind: 'all_pages' }, include_run_created_slides: true, revision: 1 },
    } });
    append(start('r1'));
    const call = event('r1', 'before-resume', { input_tokens: 1000, output_tokens: 200, total_tokens: 1200 });
    append(call);
    append({ event: 'run.canceled', data: { schema_version: 6, run_id: 'r1', occurred_at: '2026-11-01T00:00:00Z', duration_ms: 10, affected_targets: [], error: null, trace_id: 'r1' } });
    append({ event: 'run.resumed', data: { schema_version: 6, run_id: 'r1', occurred_at: '2026-11-01T00:00:01Z' } });
    append(call);
    append(start('r2'));
    append(event('r1', 'late', { input_tokens: 100, output_tokens: 20, total_tokens: 120 }));
    append(event('r2', 'new-run', { input_tokens: 5, output_tokens: 5, total_tokens: 10 }));
    const restored = hydrateRunFromHistory(records);
    expect(restored.session.activeRunId).toBe('r2');
    expect(restored.lastEventId).toBe('6');
    expect(restored.session.tokenUsageByRun.r1.total_tokens).toBe(1320);
    expect(restored.session.tokenUsageByRun.r2.total_tokens).toBe(10);
    expect(formatRunTokenUsage(restored.session.tokenUsageByRun.r1)).toBe('1,320');
    expect(restored.items.some(item => item.type === 'tool')).toBe(false);
  });
});
