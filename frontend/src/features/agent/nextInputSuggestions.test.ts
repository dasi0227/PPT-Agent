import { describe, expect, it } from 'vitest';
import type { CommandExecution } from '../../api/commands';
import type { SSEEvent } from '../../api/types';
import { nextInputShortcutIndex, reduceNextInputSuggestions, reduceSuggestCommand } from './nextInputSuggestions';

const base = { schema_version: 6 as const, run_id: 'run-1', occurred_at: '2026-09-14T00:00:00Z' };
const event = (name: SSEEvent['event'], data: Record<string, unknown> = {}) => ({
  event: name,
  data: { ...base, ...data },
} as SSEEvent);

describe('next input suggestion state', () => {
  it('activates command results only for the current idle run and latest attempt', () => {
    const command = (attempt: string, status: 'accepted' | 'completed', requests: string[] = []): CommandExecution<{ requests: string[] }> => ({
      command_id: attempt, attempt_id: attempt, attempt_no: 1, thread_id: 't1', project_id: 'p1', run_id: 'run-1',
      kind: 'suggest', source: 'automatic', status, phase: 0, input: {}, result: { requests }, created_at: 1, updated_at: 1,
    });
    let state = reduceSuggestCommand(null, command('a', 'accepted'), 'run-1', 'done');
    expect(state?.status).toBe('pending');
    state = reduceSuggestCommand(state, command('b', 'accepted'), 'run-1', 'done');
    expect(reduceSuggestCommand(state, command('a', 'completed', ['旧候选']), 'run-1', 'done')).toBe(state);
    state = reduceSuggestCommand(state, command('b', 'completed', ['优化第 2 页']), 'run-1', 'done');
    expect(state).toMatchObject({ attemptId: 'b', status: 'eligible', items: ['优化第 2 页'] });
    expect(reduceSuggestCommand(state, command('b', 'completed', ['迟到']), 'run-2', 'running')).toBe(state);
    expect(reduceNextInputSuggestions(state, event('run.started', { run_id: 'run-2' }))).toBeNull();
    expect(reduceSuggestCommand(null, command('c', 'accepted'), 'run-1', 'running')).toBeNull();
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
