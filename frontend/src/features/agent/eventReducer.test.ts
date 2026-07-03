import { describe, it, expect } from 'vitest';
import { reduceSSEEvent, TimelineItem } from './eventReducer';
import { SSEEvent } from '../../api/types';

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
});
