import { describe, expect, it } from 'vitest';
import { parseSSEEvent, SSE_EVENT_NAMES } from './sse';

const payloads: Record<string, unknown> = {
  'strategy.selected': { strategy: 'direct_action' },
  'plan.created': { plan: { id: 'p1', steps: [] } },
  'tool.called': { call_id: 'c1', tool: 'write' },
  'tool.completed': { call_id: 'c1', ok: true },
  'artifact.staged': { artifact: { kind: 'blueprint_slide', id: 's1' } },
  'artifact.committed': { artifact: { kind: 'blueprint_slide', id: 's1' } },
  needs_input: { id: 'question-1', prompt: '选择方向' },
};

describe('SSE parser', () => {
  it('parses every declared event into a discriminated event', () => {
    for (const eventName of SSE_EVENT_NAMES) {
      const event = parseSSEEvent(eventName, JSON.stringify(payloads[eventName] ?? {}), '12');
      expect(event, eventName).toMatchObject({ id: '12', event: eventName });
    }
  });

  it('rejects unknown, malformed, and incomplete events without throwing', () => {
    expect(parseSSEEvent('future.event', '{}')).toBeNull();
    expect(parseSSEEvent('run.started', '{')).toBeNull();
    expect(parseSSEEvent('tool.called', '{"call_id":"c1"}')).toBeNull();
  });
});
