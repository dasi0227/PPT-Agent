import { describe, expect, it } from 'vitest';
import { isPreviewCommand, parseRuntimeEvent, runtimeEventFromFrame } from './previewProtocol';

describe('preview protocol validation', () => {
  it('accepts only declared commands with schema-valid payloads', () => {
    expect(isPreviewCommand({
      type: 'updateDeck',
      slides: [{ id: 's1', html: '<h1>one</h1>', frame: { slide_id: 's1', canvas: { width: 1920, height: 1080, aspect_ratio: '16:9' }, theme_id: 'swiss-modern', ordinal: 1, total: 1 } }],
      index: 0,
    })).toBe(true);
    expect(isPreviewCommand({ type: 'gotoSlide', index: 2 })).toBe(true);
    expect(isPreviewCommand({ type: 'updateDeck', slides: [{ id: 's1', url: '/secret' }], index: 0 })).toBe(false);
    expect(isPreviewCommand({ type: 'executeScript', callback: 'x' })).toBe(false);
    expect(isPreviewCommand({ type: 'setSelectionMode', session_id: 'session-one', slide_id: 's1', mode: 'region', html_revision: 2, html_hash: 'sha256:a' })).toBe(true);
    expect(isPreviewCommand({ type: 'setSelectionMode', session_id: 'session-one', slide_id: 's1', mode: 'bad', html_revision: 2, html_hash: 'sha256:a' })).toBe(false);
  });

  it('accepts only declared runtime events', () => {
    expect(parseRuntimeEvent({ type: 'runtimeReady' })).toEqual({ type: 'runtimeReady' });
    expect(parseRuntimeEvent({ type: 'renderError', message: 'bad', index: 1 })).toEqual({
      type: 'renderError',
      message: 'bad',
      index: 1,
    });
    expect(parseRuntimeEvent({ type: 'renderError', message: 42 })).toBeNull();
    expect(parseRuntimeEvent({ type: 'arbitraryCallback', fn: 'x' })).toBeNull();
    const selection = { kind: 'element', slide_id: 's1', html_revision: 1, html_hash: 'sha256:a', canvas: { width: 1920, height: 1080 }, status: 'active', rect: { x: 0, y: 0, width: 10, height: 10 }, dom_targets: [], chrome_targets: [{ type: 'page_number', placement: 'bottom-right', style: '', text: '1', rect: { x: 0, y: 0, width: 10, height: 10 } }] };
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
