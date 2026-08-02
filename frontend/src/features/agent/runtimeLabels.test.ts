import { describe, expect, it } from 'vitest';
import { runStatusLabels, targetLabel } from './runtimeLabels';

describe('runtime product labels', () => {
  it('exposes only target and product run status labels', () => {
    expect(targetLabel('presentation', 'slide')).toBe('当前页HTML');
    expect(runStatusLabels.waiting).toBe('等待回答');
    expect(runStatusLabels.done).toBe('已完成');
  });
});
