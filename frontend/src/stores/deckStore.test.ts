import { beforeEach, describe, expect, test } from 'vitest';
import { useDeckStore } from './deckStore';

describe('deckStore globalView', () => {
  beforeEach(() => {
    useDeckStore.setState({ globalView: 'html' });
  });

  test('defaults to html', () => {
    const s = useDeckStore.getState();
    expect(s.globalView).toBe('html');
  });

  test('selects outline', () => {
    useDeckStore.getState().setGlobalView('outline');
    const s = useDeckStore.getState();
    expect(s.globalView).toBe('outline');
  });

  test('switches from outline back to html', () => {
    useDeckStore.getState().setGlobalView('outline');
    useDeckStore.getState().setGlobalView('html');
    expect(useDeckStore.getState().globalView).toBe('html');
  });

});
