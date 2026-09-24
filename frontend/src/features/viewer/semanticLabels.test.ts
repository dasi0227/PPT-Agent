import { describe, expect, it } from 'vitest';
import { decorationPlacementLabel, decorationTypeLabel, elementTypeLabel, partLabel, slideRoleLabel } from './semanticLabels';

describe('semanticLabels', () => {
  it('maps known slide roles to Chinese labels and falls back on unknown', () => {
    expect(slideRoleLabel('cover')).toBe('封面');
    expect(slideRoleLabel('EVIDENCE')).toBe('论据');
    expect(slideRoleLabel('how-to')).toBe('操作指引');
    expect(slideRoleLabel('example')).toBe('案例');
    expect(slideRoleLabel('conclusion')).toBe('结论');
    expect(slideRoleLabel('mystery-role')).toBe('内容');
  });

  it('maps resource parts', () => {
    expect(partLabel('design')).toBe('视觉要求');
    expect(partLabel('html')).toBe('幻灯片');
    expect(partLabel('unknown')).toBe('演示内容');
  });

  it('maps element types to Chinese labels and falls back on unknown', () => {
    expect(elementTypeLabel('text')).toBe('文本');
    expect(elementTypeLabel('chart')).toBe('图表');
    expect(elementTypeLabel('asset')).toBe('素材');
    expect(elementTypeLabel('unknown-type')).toBe('内容元素');
  });

  it('describes decoration names, placements and disabled state', () => {
    expect(decorationTypeLabel('page_number')).toBe('页码');
    expect(decorationTypeLabel('section_title')).toBe('章节标题');
    expect(decorationPlacementLabel('bottom-right')).toBe('右下');
    expect(decorationPlacementLabel('none')).toBe('暂不展示');
  });
});
