import { describe, expect, it } from 'vitest';
import { presentUserText, runStatusLabels, targetLabel } from './runtimeLabels';

describe('runtime product labels', () => {
  it('exposes only target and product run status labels', () => {
    expect(targetLabel('presentation', 'slide')).toBe('当前页HTML');
    expect(targetLabel('spec', 'deck')).toBe('整份设计稿');
    expect(presentUserText('已读取全局蓝图')).toBe('已读取全局设计稿');
    expect(runStatusLabels.waiting).toBe('等待回答');
    expect(runStatusLabels.done).toBe('已完成');
  });
});
