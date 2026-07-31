import { describe, expect, test } from 'vitest';
import { hydrateFromHistory, HistoryEntry } from './historyHydrator';

describe('hydrateFromHistory', () => {
  test('sorts by seq and maps user_turn / markdown / final_result', () => {
    const entries: HistoryEntry[] = [
      { seq: 2, ts: 200, run_id: 'r1', turn: 'agent', type: 'markdown', data: { text: 'reply' } },
      { seq: 1, ts: 100, run_id: 'r1', turn: 'user', type: 'user_turn', data: { text: 'hi' } },
      { seq: 3, ts: 300, run_id: 'r1', turn: 'agent', type: 'final_result', data: { result: { ok: true } } },
    ];
    const items = hydrateFromHistory(entries);
    expect(items).toHaveLength(3);
    expect(items[0].type).toBe('user_turn');
    expect((items[0] as any).text).toBe('hi');
    expect(items[1].type).toBe('markdown');
    expect((items[1] as any).text).toBe('reply');
    expect(items[2].type).toBe('final_result');
    expect((items[2] as any).result).toEqual({ ok: true });
  });

  test('maps info to markdown and needs_input keeps id from data', () => {
    const entries: HistoryEntry[] = [
      { seq: 1, ts: 0, run_id: 'r', turn: 'agent', type: 'info', data: { text: 'note' } },
      { seq: 2, ts: 0, run_id: 'r', turn: 'agent', type: 'needs_input', data: { id: 'ni_1', prompt: 'go?', choices: ['A'] } },
    ];
    const items = hydrateFromHistory(entries);
    expect(items[0].type).toBe('markdown');
    expect((items[0] as any).text).toBe('note');
    expect(items[1].type).toBe('needs_input');
    expect(items[1].id).toBe('ni_1');
    expect((items[1] as any).prompt).toBe('go?');
    expect((items[1] as any).choices).toEqual(['A']);
  });

  test('maps error type', () => {
    const items = hydrateFromHistory([
      { seq: 1, ts: 0, run_id: 'r', turn: 'agent', type: 'error', data: { code: 'BAD', message: 'boom' } },
    ]);
    expect(items[0].type).toBe('error');
    expect((items[0] as any).code).toBe('BAD');
    expect((items[0] as any).message).toBe('boom');
  });

  test('drops unknown types', () => {
    const items = hydrateFromHistory([
      { seq: 1, ts: 1, run_id: 'r', turn: 'agent', type: 'weird' as any, data: {} },
    ]);
    expect(items).toHaveLength(0);
  });

  test('empty entries returns empty array', () => {
    expect(hydrateFromHistory([])).toEqual([]);
  });

  test('replays concise context assembled status', () => {
    const items = hydrateFromHistory([
      { seq: 1, ts: 2, run_id: 'r', turn: 'agent', type: 'context_assembled', data: {
        profile: 'blueprint/deck', warnings: [], read_only: false,
      } },
    ]);
    expect(items[0]).toMatchObject({ type: 'context_status', profile: 'blueprint/deck', warnings: [], readOnly: false });
  });
});
