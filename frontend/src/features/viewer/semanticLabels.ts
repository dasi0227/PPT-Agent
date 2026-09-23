import type { Design } from '../../api/types';

// 结构化枚举字段 -> 人类可读中文标签的统一映射层。
// 目的：面向用户的展示层永远不直接渲染内部字段值（role/part/chrome.type 等），
// 所有映射集中在此，避免跨组件双写。未知枚举使用中文回退名称；自由文本不在此翻译。

type ChromeItem = Design['chrome'][number];

// 幻灯片语义角色（outline slide node 的 role 字段）。
const SLIDE_ROLE_LABELS: Record<string, string> = {
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
  manifest: '演示要求',
  outline: '目录结构',
  design: '全局设计',
  spec: '页面设计稿',
  html: '幻灯片',
};

// 页面装饰件类型（chrome.type）。
const CHROME_TYPE_LABELS: Record<ChromeItem['type'], string> = {
  page_number: '页码',
  section_marker: '章节标记',
  key_message: '核心信息',
  deck_title: '演示标题',
};

// 页面装饰件位置（chrome.placement）。
const CHROME_PLACEMENT_LABELS: Record<ChromeItem['placement'], string> = {
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

export function slideRoleLabel(role: string): string {
  return SLIDE_ROLE_LABELS[role.trim().toLowerCase()] ?? '内容';
}

export function partLabel(part: string): string {
  return PART_LABELS[part.trim().toLowerCase()] ?? '演示内容';
}

export function elementTypeLabel(type: string): string {
  return ELEMENT_TYPE_LABELS[type.trim().toLowerCase()] ?? '内容元素';
}

export function chromeLabel(item: ChromeItem): string {
  const type = CHROME_TYPE_LABELS[item.type] ?? '页面装饰';
  const placement = CHROME_PLACEMENT_LABELS[item.placement] ?? '自定义位置';
  return `${type}（${placement}）`;
}
