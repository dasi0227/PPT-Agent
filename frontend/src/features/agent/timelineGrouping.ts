import type { TimelineGroup } from '../../api/session';
import { isResourceEditTool } from '../../api/resourceTools';
import type { FinalMessageItem, TerminalNoticeItem, TimelineItem, ToolActivityItem } from './timelineTypes';

export type DisplayEntry =
  | { kind: 'group_page'; id: string; groupId: string }
  | { kind: 'item'; item: TimelineItem }
  | { kind: 'tool_group'; id: string; items: ToolActivityItem[] }
  | {
      kind: 'run_summary';
      id: string;
      runId: string;
      status: 'completed' | 'failed' | 'error' | 'canceled' | 'paused';
      groupId: string;
      durationMs: number;
      terminalItem?: FinalMessageItem | TerminalNoticeItem;
      processEntries: DisplayEntry[];
    };

function canGroupTool(item: ToolActivityItem): boolean {
  return !item.detailRef && item.interactionState !== 'pending' && item.interactionState !== 'answered'
    && item.status === 'completed'
    && (item.target?.type !== 'deck' || isResourceEditTool(item.tool))
    && !item.error
    && item.approval?.answer?.decision !== 'reject'
    && item.command?.status !== 'blocked';
}

function sameToolGroup(first: ToolActivityItem, next: ToolActivityItem): boolean {
  if (isResourceEditTool(first.tool) && isResourceEditTool(next.tool)) {
    return first.runId === next.runId;
  }
  return first.tool === next.tool
    && first.target?.part === next.target?.part
    && (first.tool !== 'read_image' || first.image?.source === next.image?.source);
}

function groupToolItems(items: TimelineItem[]): DisplayEntry[] {
  const result: DisplayEntry[] = [];
  let pending: ToolActivityItem[] = [];

  const flush = () => {
    if (pending.length >= 2) {
      result.push({ kind: 'tool_group', id: `group:${pending[0].id}`, items: pending });
    } else {
      pending.forEach((item) => result.push({ kind: 'item', item }));
    }
    pending = [];
  };

  for (const item of items) {
    if (item.type !== 'tool' || !canGroupTool(item)) {
      flush();
      result.push({ kind: 'item', item });
      continue;
    }
    const first = pending[0];
    // 同轮连续成功编辑跨资源成组；其他工具沿用工具、资源类型及图片来源分组。
    if (first && !sameToolGroup(first, item)) {
      flush();
    }
    pending.push(item);
  }
  flush();
  return result;
}

export function visibleTimelineItems(items: TimelineItem[], showToolFailures: boolean): TimelineItem[] {
  return showToolFailures ? items : items.filter(item => item.type !== 'tool' || item.interactionState === 'pending' || item.interactionState === 'answered' || !['failed', 'blocked', 'interrupted', 'canceled'].includes(item.status));
}

export function groupTimelineItems(items: TimelineItem[], groups: TimelineGroup[]): DisplayEntry[] {
  const result: DisplayEntry[] = [];
  const consumed = new Set<string>();
  for (const group of groups) {
    const entries = items.filter(item => item.groupId === group.id);
    entries.forEach(item => consumed.add(item.id));
    if (group.kind !== 'run' || !group.run_id || !['done', 'failed', 'canceled', 'paused'].includes(group.status)) {
      result.push({ kind: 'group_page', id: `page:${group.id}`, groupId: group.id }, ...groupToolItems(entries));
      continue;
    }
    const terminal = [...entries].reverse().find((item): item is FinalMessageItem | TerminalNoticeItem =>
      group.status === 'done' ? item.type === 'final' : item.type === 'terminal_notice');
    const final = terminal ?? [...entries].reverse().find((item): item is FinalMessageItem => item.type === 'final');
    const publicInputs = entries.filter(item => item.type === 'user_turn' || item.type === 'input_reply');
    const process = entries.filter(item => !['user_turn', 'input_reply', 'final', 'terminal_notice'].includes(item.type));
    result.push(...publicInputs.map(item => ({ kind: 'item' as const, item })));
    result.push({ kind: 'run_summary', id: group.id, groupId: group.id, runId: group.run_id,
      status: group.status === 'done' ? 'completed' : group.status as 'failed' | 'canceled' | 'paused',
      durationMs: typeof group.summary.active_duration_ms === 'number' ? group.summary.active_duration_ms : 0,
      terminalItem: final, processEntries: groupToolItems(process) });
  }
  // Current interactions and local pending input can be outside the loaded page.
  result.push(...groupToolItems(items.filter(item => !consumed.has(item.id))));
  return result;
}
