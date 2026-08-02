import type { TimelineItem, ToolActivityItem } from './eventReducer';

export type DisplayEntry =
  | { kind: 'item'; item: TimelineItem }
  | { kind: 'tool_group'; id: string; items: ToolActivityItem[] };

function canGroupTool(item: ToolActivityItem, currentSlideId?: string): boolean {
  return item.status === 'completed'
    && !item.error
    && (item.preview?.warnings.length ?? 0) === 0
    && item.target?.slide_id !== currentSlideId;
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
    if (first && (first.tool !== item.tool || first.planStepId !== item.planStepId)) {
      flush();
    }
    pending.push(item);
  }
  flush();
  return result;
}
