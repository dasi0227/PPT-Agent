import { describe, it, expect } from 'vitest';
import { isMac } from './platform';

describe('platform', () => {
  it('returns false when navigator is undefined', () => {
    const originalNavigator = global.navigator;
    Object.defineProperty(global, 'navigator', {
      value: undefined,
      configurable: true,
    });
    expect(isMac()).toBe(false);
    Object.defineProperty(global, 'navigator', {
      value: originalNavigator,
      configurable: true,
    });
  });

  it('detects Mac correctly', () => {
    const originalNavigator = global.navigator;
    Object.defineProperty(global, 'navigator', {
      value: { platform: 'MacIntel' },
      configurable: true,
    });
    expect(isMac()).toBe(true);
    Object.defineProperty(global, 'navigator', {
      value: originalNavigator,
      configurable: true,
    });
  });
});
