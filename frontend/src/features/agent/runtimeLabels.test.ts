import { describe, expect, it } from 'vitest';
import { presentUserText, runStatusLabels, targetLabel } from './runtimeLabels';

describe('runtime product labels', () => {
  it('exposes only target and product run status labels', () => {
    expect(targetLabel({ object: 'html', selection: { kind: 'current_page' } })).toBe('当前页 · 幻灯片');
    expect(targetLabel({ object: 'spec', selection: { kind: 'all_pages' } })).toBe('全部页 · 设计稿');
    expect(presentUserText('已读取全局蓝图')).toBe('已读取全局设计稿');
    expect(runStatusLabels.waiting).toBe('等待回答');
    expect(runStatusLabels.done).toBe('已完成');
  });
});
