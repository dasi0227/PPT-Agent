import type { DecorationPlacement, DecorationType, SlideRole } from '../../api/types';

// 结构化枚举字段 -> 人类可读中文标签的统一映射层。
// 目的：面向用户的展示层永远不直接渲染内部字段值（role/part/装饰键等），
// 所有映射集中在此，避免跨组件双写。未知枚举使用中文回退名称；自由文本不在此翻译。

// 幻灯片语义角色（SlideSpec 的可选 role 字段）。
const SLIDE_ROLE_LABELS: Record<SlideRole, string> = {
  cover: '封面',
  agenda: '目录',
  context: '背景',
  content: '内容',
  definition: '定义',
  evidence: '论据',
  comparison: '对比',
  example: '案例',
  'how-to': '操作指引',
  transition: '过渡',
  summary: '总结',
  conclusion: '结论',
};

// 资源部位（PublicTarget.part）。
const PART_LABELS: Record<string, string> = {
  manifest: '内容要求',
  outline: '目录结构',
  design: '视觉要求',
  spec: '规格要求',
  html: '幻灯片',
};

// 页面装饰对象的固定键。
const DECORATION_TYPE_LABELS: Record<DecorationType, string> = {
  page_number: '页码',
  section_title: '章节标题',
  key_message: '核心信息',
  deck_title: '演示标题',
};

// 页面装饰件位置。
const DECORATION_PLACEMENT_LABELS: Record<DecorationPlacement | 'none', string> = {
  none: '暂不展示',
  'top-left': '左上',
  'top-center': '顶部居中',
  'top-right': '右上',
  'bottom-left': '左下',
  'bottom-center': '底部居中',
  'bottom-right': '右下',
  'left-edge': '左侧边',
  'right-edge': '右侧边',
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

export const slideRoleOptions = Object.entries(SLIDE_ROLE_LABELS).map(([value, label]) => ({ value, label }));

export function slideRoleLabel(role?: string): string {
  if (!role?.trim()) return '未设置';
  return SLIDE_ROLE_LABELS[role.trim().toLowerCase() as SlideRole] ?? '内容';
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

export function decorationPlacementLabel(placement: DecorationPlacement | 'none'): string {
  return DECORATION_PLACEMENT_LABELS[placement] ?? '自定义位置';
}
