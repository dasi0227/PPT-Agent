import { beforeEach, describe, expect, test } from 'vitest';
import { useDeckStore } from './deckStore';

describe('deckStore per-page view preference', () => {
  beforeEach(() => {
    useDeckStore.setState({ viewByPage: {} });
  });

  test('effectiveView smart default and override', () => {
    const s = useDeckStore.getState();
    expect(s.effectiveView('s1', true)).toBe('html');
    expect(s.effectiveView('s2', false)).toBe('outline');
    s.setPageView('s2', 'html'); // 覆盖：即便无 html 也返回 html（UI 层禁用不可选）
    expect(useDeckStore.getState().effectiveView('s2', false)).toBe('html');
    s.setPageView('s1', 'outline'); // 有 html 也可手动切回大纲
    expect(useDeckStore.getState().effectiveView('s1', true)).toBe('outline');
  });
});
