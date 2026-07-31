import { describe, expect, it } from 'vitest';
import { isPreviewCommand, parseRuntimeEvent, runtimeEventFromFrame } from './previewProtocol';

describe('preview protocol validation', () => {
  it('accepts only declared commands with schema-valid payloads', () => {
    expect(isPreviewCommand({
      type: 'updateDeck',
      slides: [{ id: 's1', html: '<h1>one</h1>' }],
      index: 0,
    })).toBe(true);
    expect(isPreviewCommand({ type: 'gotoSlide', index: 2 })).toBe(true);
    expect(isPreviewCommand({ type: 'updateDeck', slides: [{ id: 's1', url: '/secret' }], index: 0 })).toBe(false);
    expect(isPreviewCommand({ type: 'executeScript', callback: 'x' })).toBe(false);
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
