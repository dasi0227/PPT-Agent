import { describe, expect, it } from 'vitest';
import { parseRuntimeEvent, runtimeEventFromFrame } from './previewProtocol';

describe('preview protocol validation', () => {
  it('accepts only declared runtime events', () => {
    expect(parseRuntimeEvent({ type: 'runtimeReady' })).toEqual({ type: 'runtimeReady' });
    expect(parseRuntimeEvent({ type: 'renderError', message: 'bad', index: 1 })).toEqual({
      type: 'renderError',
      message: 'bad',
      index: 1,
    });
    expect(parseRuntimeEvent({ type: 'renderError', message: 42 })).toBeNull();
    expect(parseRuntimeEvent({ type: 'arbitraryCallback', fn: 'x' })).toBeNull();
    const selection = { kind: 'element', slide_id: 's1', html_hash: 'sha256:a', canvas: { width: 1920, height: 1080 }, status: 'active', rect: { x: 0, y: 0, width: 10, height: 10 }, dom_targets: [], decoration_targets: [{ type: 'page_number', placement: 'bottom-right', text: '1', rect: { x: 0, y: 0, width: 10, height: 10 } }] };
    expect(parseRuntimeEvent({ type: 'selectionCreated', session_id: 'session-one', slide_id: 's1', selection })?.type).toBe('selectionCreated');
    expect(parseRuntimeEvent({ type: 'selectionCreated', session_id: 'session-one', slide_id: 's1', selection: { ...selection, rect: { x: -1, y: 0, width: 10, height: 10 } } })).toBeNull();
  });

  it('rejects valid-looking events from any source other than the active iframe', () => {
    const activeFrame = {} as Window;
    expect(runtimeEventFromFrame(
      { source: {} as Window, data: { type: 'runtimeReady' } } as MessageEvent,
      activeFrame,
    )).toBeNull();
    expect(runtimeEventFromFrame(
      { source: activeFrame, data: { type: 'runtimeReady' } } as MessageEvent,
      activeFrame,
    )).toEqual({ type: 'runtimeReady' });
  });
});
