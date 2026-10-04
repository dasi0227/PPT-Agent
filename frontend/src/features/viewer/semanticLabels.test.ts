import { describe, expect, it } from 'vitest';
import { decorationPlacementLabel, decorationTypeLabel, elementTypeLabel, partLabel, slideContentTypeLabel, slidePurposeLabel } from './semanticLabels';

describe('semanticLabels', () => {
  it('maps page purposes and content types to Chinese labels and leaves unknown unset', () => {
    expect(slidePurposeLabel('cover')).toBe('封面');
    expect(slidePurposeLabel('INTRODUCTION')).toBe('引入');
    expect(slideContentTypeLabel('guidance')).toBe('指引');
    expect(slideContentTypeLabel('example')).toBe('示例');
    expect(slidePurposeLabel('conclusion')).toBe('结论');
    expect(slidePurposeLabel('mystery-purpose')).toBe('未设置');
  });

  it('maps resource parts', () => {
    expect(partLabel('design')).toBe('视觉要求');
    expect(partLabel('spec')).toBe('设计稿');
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
