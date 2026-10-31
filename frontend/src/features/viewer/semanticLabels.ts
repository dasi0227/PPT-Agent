import type { DecorationType, SlidePurpose } from '../../api/types';

// 结构化枚举字段 -> 人类可读中文标签的统一映射层。
// 目的：面向用户的展示层永远不直接渲染内部字段值（purpose/part/装饰键等），
// 所有映射集中在此，避免跨组件双写。未知枚举使用中文回退名称；自由文本不在此翻译。

// 幻灯片页面用途（SlideSpec 的 purpose 字段）。
const SLIDE_PURPOSE_LABELS: Record<SlidePurpose, string> = {
  cover: '封面',
  introduction: '引入',
  transition: '过渡',
  content_explanation: '正文·解释',
  content_comparison: '正文·对比',
  content_example: '正文·示例',
  content_guidance: '正文·指引',
  content_other: '正文·其他',
  conclusion: '结论',
  other: '其他',
};

// 资源部位（PublicTarget.part）。
const PART_LABELS: Record<string, string> = {
  manifest: '内容要求',
  outline: '目录结构',
  design: '视觉要求',
  spec: '设计稿',
  html: '幻灯片',
};

// 页面装饰对象的固定键。
const DECORATION_TYPE_LABELS: Record<DecorationType, string> = {
  slide_pagination: '页码',
  section_title: '章节标题',
  logo: '演示标志',
  deck_title: '演示标题',
};

// 页面元素类型（SlideSpec.elements[].type）。
const ELEMENT_TYPE_LABELS: Record<string, string> = {
  text: '文本',
  list: '列表',
  metric: '指标',
  quote: '引用',
  table: '表格',
  chart: '图表',
  diagram: '图示',
  code: '代码',
  asset: '素材',
};

export const slidePurposeOptions = Object.entries(SLIDE_PURPOSE_LABELS).map(([value, label]) => ({ value, label }));

export function slidePurposeLabel(purpose?: string): string {
  if (!purpose?.trim()) return '未设置';
  return SLIDE_PURPOSE_LABELS[purpose.trim().toLowerCase() as SlidePurpose] ?? '未设置';
}

export function partLabel(part: string): string {
  return PART_LABELS[part.trim().toLowerCase()] ?? '演示内容';
}

export function elementTypeLabel(type: string): string {
  return ELEMENT_TYPE_LABELS[type.trim().toLowerCase()] ?? '内容元素';
}

export function decorationTypeLabel(type: DecorationType): string {
  return DECORATION_TYPE_LABELS[type] ?? '页面装饰';
}

