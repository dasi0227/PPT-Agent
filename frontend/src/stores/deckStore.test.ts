import { beforeEach, describe, expect, test } from 'vitest';
import { useDeckStore } from './deckStore';

describe('deckStore globalView', () => {
  beforeEach(() => {
    useDeckStore.setState({ globalView: 'html' });
  });

  test('default is html and stays html even when hasHtml=false', () => {
    const s = useDeckStore.getState();
    expect(s.globalView).toBe('html');
    expect(s.effectiveView('s1', true)).toBe('html');
    expect(s.effectiveView('s2', false)).toBe('html');
  });

  test('globalView=outline forces outline regardless of hasHtml', () => {
    useDeckStore.getState().setGlobalView('outline');
    const s = useDeckStore.getState();
    expect(s.effectiveView('s1', true)).toBe('outline');
    expect(s.effectiveView('s2', false)).toBe('outline');
  });

  test('setGlobalView(html) restores default and picks html when available', () => {
    useDeckStore.getState().setGlobalView('outline');
    useDeckStore.getState().setGlobalView('html');
    expect(useDeckStore.getState().globalView).toBe('html');
    expect(useDeckStore.getState().effectiveView('s1', true)).toBe('html');
    expect(useDeckStore.getState().effectiveView('s2', false)).toBe('html');
  });

});
