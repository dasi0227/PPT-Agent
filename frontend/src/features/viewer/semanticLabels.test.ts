import { describe, expect, it } from 'vitest';
import { chromeLabel, elementTypeLabel, partLabel, slideRoleLabel } from './semanticLabels';

describe('semanticLabels', () => {
  it('maps known slide roles to Chinese labels and falls back on unknown', () => {
    expect(slideRoleLabel('cover')).toBe('封面');
    expect(slideRoleLabel('EVIDENCE')).toBe('论据');
    expect(slideRoleLabel('how-to')).toBe('操作指引');
    expect(slideRoleLabel('example')).toBe('案例');
    expect(slideRoleLabel('conclusion')).toBe('结论');
    expect(slideRoleLabel('mystery-role')).toBe('mystery-role');
  });

  it('maps resource parts', () => {
    expect(partLabel('design')).toBe('全局设计');
    expect(partLabel('html')).toBe('幻灯片');
    expect(partLabel('unknown')).toBe('unknown');
  });

  it('maps element types to Chinese labels and falls back on unknown', () => {
    expect(elementTypeLabel('text')).toBe('文本');
    expect(elementTypeLabel('chart')).toBe('图表');
    expect(elementTypeLabel('asset')).toBe('素材');
    expect(elementTypeLabel('unknown-type')).toBe('unknown-type');
  });

  it('describes chrome item as type（placement）', () => {
    expect(chromeLabel({ type: 'page_number', placement: 'bottom-right', style: 'x' })).toBe('页码（右下）');
    expect(chromeLabel({ type: 'section_marker', placement: 'top-left', style: 'x' })).toBe('章节标记（左上）');
  });
});
