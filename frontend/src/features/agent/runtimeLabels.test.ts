import { describe, expect, it } from 'vitest';
import { runActivityLabels, runStatusLabels, targetLabel } from './runtimeLabels';

describe('runtime product labels', () => {
  it('exposes only target and product run status labels', () => {
    expect(targetLabel({ selection: { kind: 'current_page' } })).toBe('当前页');
    expect(targetLabel({ selection: { kind: 'all_pages' } })).toBe('全部页');
    expect(runStatusLabels.waiting).toBe('等待回答');
    expect(runStatusLabels.done).toBe('已完成');
  });

  it('maps visible run activities to Dasi-authored user-facing copy', () => {
    expect(runActivityLabels['run.analyzing']).toBe('Dasi 正在推进任务');
    expect(runActivityLabels['slide.layout.checking']).toBe('Dasi 正在渲染幻灯片');
    expect(Object.values(runActivityLabels).every((label) => label === null || label.startsWith('Dasi '))).toBe(true);
  });
});
