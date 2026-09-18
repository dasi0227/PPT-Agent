import { describe, expect, it } from 'vitest';
import type { SSEEvent } from '../../api/types';
import { nextInputShortcutIndex, reduceNextInputSuggestions } from './nextInputSuggestions';

const base = { schema_version: 5 as const, run_id: 'run-1', occurred_at: '2026-09-14T00:00:00Z' };
const event = (name: SSEEvent['event'], data: Record<string, unknown> = {}) => ({
  event: name,
  data: { ...base, ...data },
} as SSEEvent);

describe('next input suggestion state', () => {
  it('stages final suggestions, activates them on completion, and consumes them on the next accepted run', () => {
    let state = reduceNextInputSuggestions(null, event('message.final', {
      message_id: 'final-1', text: '完成', affected_targets: [],
      suggested_next_inputs: ['优化第 2 页'], project_history_revision: 8,
    }));
    expect(state).toMatchObject({
      runId: 'run-1', messageId: 'final-1', status: 'staged',
      items: ['优化第 2 页'], projectHistoryRevision: 8,
    });

    state = reduceNextInputSuggestions(state, event('run.completed', {
      duration_ms: 1, affected_targets: [], error: null, trace_id: 'run-1',
    }));
    expect(state?.status).toBe('eligible');

    state = reduceNextInputSuggestions(state, event('run.started', {
      run_id: 'run-2', scope: {}, mode: 'execute', user_input: '继续',
    }));
    expect(state).toBeNull();
    expect(reduceNextInputSuggestions(state, event('run.failed', {
      run_id: 'run-2', duration_ms: 1, affected_targets: [], error: {}, trace_id: 'run-2',
    }))).toBeNull();
  });

  it('ignores late final and terminal events from a run that is no longer active', () => {
    const current = {
      runId: 'run-2', messageId: 'final-2', items: ['检查整套叙事'],
      projectHistoryRevision: 9, status: 'eligible' as const,
    };
    const lateFinal = event('message.final', {
      message_id: 'final-1', text: '旧结果', affected_targets: [],
      suggested_next_inputs: ['旧候选'], project_history_revision: 8,
    });
    const lateFailure = event('run.failed', {
      duration_ms: 1, affected_targets: [], error: {}, trace_id: 'run-1',
    });
    expect(reduceNextInputSuggestions(current, lateFinal, 'run-2')).toBe(current);
    expect(reduceNextInputSuggestions(current, lateFailure, 'run-2')).toBe(current);
  });

  it('only accepts unmodified Alt/Option digit shortcuts outside conflicting UI', () => {
    const valid = {
      altKey: true, metaKey: false, ctrlKey: false, shiftKey: false,
      isComposing: false, code: 'Digit2', blocked: false,
      targetEditable: false, targetIsComposer: false,
    };
    expect(nextInputShortcutIndex(valid)).toBe(1);
    expect(nextInputShortcutIndex({ ...valid, isComposing: true })).toBeNull();
    expect(nextInputShortcutIndex({ ...valid, blocked: true })).toBeNull();
    expect(nextInputShortcutIndex({ ...valid, targetEditable: true })).toBeNull();
    expect(nextInputShortcutIndex({ ...valid, targetEditable: true, targetIsComposer: true })).toBe(1);
    expect(nextInputShortcutIndex({ ...valid, metaKey: true })).toBeNull();
    expect(nextInputShortcutIndex({ ...valid, code: 'Numpad2' })).toBeNull();
  });
});
