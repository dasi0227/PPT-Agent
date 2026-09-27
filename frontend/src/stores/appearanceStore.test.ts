import { readFileSync } from 'node:fs';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { COLOR_MODE_KEY, useAppearanceStore } from './appearanceStore';

const bootstrap = readFileSync('index.html', 'utf8').match(/<script>([\s\S]*?)<\/script>/)![1];

beforeEach(() => {
  vi.restoreAllMocks();
  localStorage.clear();
  document.documentElement.dataset.colorMode = 'light';
  useAppearanceStore.setState({ colorMode: 'light' });
});

describe('editor appearance', () => {
  it('applies the saved appearance before React loads, with a light default for unknown values', () => {
    localStorage.setItem(COLOR_MODE_KEY, 'dark');
    new Function(bootstrap)();
    expect(document.documentElement.dataset.colorMode).toBe('dark');
    localStorage.setItem(COLOR_MODE_KEY, 'unexpected');
    new Function(bootstrap)();
    expect(document.documentElement.dataset.colorMode).toBe('light');
  });

  it('persists a reversible change and leaves embedded presentation documents alone', () => {
    const frame = document.createElement('iframe');
    document.body.append(frame);
    frame.contentDocument!.documentElement.dataset.presentationTheme = 'editorial-serif';
    useAppearanceStore.getState().toggleColorMode();
    expect(document.documentElement.dataset.colorMode).toBe('dark');
    expect(localStorage.getItem(COLOR_MODE_KEY)).toBe('dark');
    expect(frame.contentDocument!.documentElement.dataset.colorMode).toBeUndefined();
    expect(frame.contentDocument!.documentElement.dataset.presentationTheme).toBe('editorial-serif');
    useAppearanceStore.getState().toggleColorMode();
    expect(localStorage.getItem(COLOR_MODE_KEY)).toBe('light');
    frame.remove();
  });

  it('synchronizes another tab’s preference, including clearing it, without reacting to unrelated settings', () => {
    window.dispatchEvent(new StorageEvent('storage', { key: COLOR_MODE_KEY, newValue: 'dark' }));
    expect(useAppearanceStore.getState().colorMode).toBe('dark');
    expect(document.documentElement.dataset.colorMode).toBe('dark');
    window.dispatchEvent(new StorageEvent('storage', { key: 'another-setting', newValue: 'light' }));
    expect(useAppearanceStore.getState().colorMode).toBe('dark');
    window.dispatchEvent(new StorageEvent('storage', { key: null, newValue: null }));
    expect(useAppearanceStore.getState().colorMode).toBe('light');
  });

  it('still starts and toggles when browser storage is unavailable', () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new Error('storage unavailable'); });
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('storage unavailable'); });
    expect(() => new Function(bootstrap)()).not.toThrow();
    expect(() => useAppearanceStore.getState().toggleColorMode()).not.toThrow();
    expect(document.documentElement.dataset.colorMode).toBe('dark');
  });
});
