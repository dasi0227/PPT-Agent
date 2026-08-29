import type { Design } from '../../api/types';

// 结构化枚举字段 -> 人类可读中文标签的统一映射层。
// 目的：面向用户的展示层永远不直接渲染内部字段值（role/part/chrome.type 等），
// 所有映射集中在此，避免跨组件双写。未知值一律回退为原值，保证不崩且可观测。

type ChromeItem = Design['chrome'][number];

// 幻灯片语义角色（slide spec 的 role 字段）。
const SLIDE_ROLE_LABELS: Record<string, string> = {
  cover: '封面',
  context: '背景',
  definition: '定义',
  evidence: '论据',
  comparison: '对比',
  examples: '案例',
  'how-to': '操作指引',
  summary: '总结',
  agenda: '目录',
  transition: '过渡',
  conclusion: '结论',
};

// 资源部位（PublicTarget.part）。
const PART_LABELS: Record<string, string> = {
  outline: '目录结构',
  design: '全局设计',
  spec: '设计稿',
  html: '幻灯片',
};

// 视觉密度（design.density）。
const DENSITY_LABELS: Record<Design['density'], string> = {
  sparse: '宽松',
  medium: '适中',
  dense: '紧凑',
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

export function slideRoleLabel(role: string): string {
  return SLIDE_ROLE_LABELS[role.trim().toLowerCase()] ?? role;
}

export function partLabel(part: string): string {
  return PART_LABELS[part.trim().toLowerCase()] ?? part;
}

export function densityLabel(density: Design['density']): string {
  return DENSITY_LABELS[density] ?? density;
}

export function chromeLabel(item: ChromeItem): string {
  const type = CHROME_TYPE_LABELS[item.type] ?? item.type;
  const placement = CHROME_PLACEMENT_LABELS[item.placement] ?? item.placement;
  return `${type}（${placement}）`;
}
