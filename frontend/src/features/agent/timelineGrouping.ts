import type { TimelineItem, ToolActivityItem } from './eventReducer';

export type DisplayEntry =
  | { kind: 'item'; item: TimelineItem }
  | { kind: 'tool_group'; id: string; items: ToolActivityItem[] };

function canGroupTool(item: ToolActivityItem, currentSlideId?: string): boolean {
  return item.status === 'completed'
    && !item.error
    && (item.preview?.warnings.length ?? 0) === 0
    // 仅当条目确有 slide_id 且正是当前查看页时才排除；deck 级（无 slide_id）始终可汇聚。
    && !(item.target?.slide_id !== undefined && item.target.slide_id === currentSlideId);
}

export function groupTimelineItems(items: TimelineItem[], currentSlideId?: string): DisplayEntry[] {
  const result: DisplayEntry[] = [];
  let pending: ToolActivityItem[] = [];

  const flush = () => {
    if (pending.length >= 3) {
      result.push({ kind: 'tool_group', id: `group:${pending[0].id}`, items: pending });
    } else {
      pending.forEach((item) => result.push({ kind: 'item', item }));
    }
    pending = [];
  };

  for (const item of items) {
    if (item.type !== 'tool' || !canGroupTool(item, currentSlideId)) {
      flush();
      result.push({ kind: 'item', item });
      continue;
    }
    const first = pending[0];
    // 纯按工具汇聚：同一 tool 的连续成功条目即可合并，跨产物（设计稿/幻灯片/deck）也归为一组。
    if (first && first.tool !== item.tool) {
      flush();
    }
    pending.push(item);
  }
  flush();
  return result;
}
