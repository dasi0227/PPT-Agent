import { isResourceEditTool } from '../../api/resourceTools';
import type { FinalMessageItem, TerminalNoticeItem, TimelineItem, ToolActivityItem } from './eventReducer';

export type DisplayEntry =
  | { kind: 'item'; item: TimelineItem }
  | { kind: 'tool_group'; id: string; items: ToolActivityItem[] }
  | {
      kind: 'run_summary';
      id: string;
      runId: string;
      status: 'completed' | 'failed' | 'error' | 'canceled';
      terminalItem: FinalMessageItem | TerminalNoticeItem;
      processEntries: DisplayEntry[];
    };

function canGroupTool(item: ToolActivityItem): boolean {
  return item.status === 'completed'
    && (item.target?.type !== 'deck' || isResourceEditTool(item.tool))
    && !item.error
    && (item.preview?.warnings.length ?? 0) === 0;
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

function terminalStatus(item: TimelineItem): 'completed' | 'failed' | 'error' | 'canceled' | null {
  if (item.type === 'final') return 'completed';
  if (item.type === 'terminal_notice') return item.status;
  return null;
}

export function visibleTimelineItems(items: TimelineItem[], showToolFailures: boolean): TimelineItem[] {
  if (showToolFailures) return items;
  return items.filter(item => {
    if (item.type === 'git_commit' && item.commandSource === 'automatic' && item.runId && item.status === 'failed') return false;
    if (item.type !== 'tool' || (item.status !== 'failed' && item.status !== 'blocked')) return true;
    // Submitted human decisions remain visible regardless of error presentation.
    return item.approval?.answer?.decision === 'reject'
      || item.error?.code === 'RESOURCE_EDIT_REJECTED'
      || item.error?.code === 'COMMAND_PERMISSION_DENIED';
  });
}

export function groupTimelineItems(items: TimelineItem[], _currentSlideId?: string): DisplayEntry[] {
  const terminalByRunId = new Map<string, FinalMessageItem | TerminalNoticeItem>();
  const processItemsByRunId = new Map<string, TimelineItem[]>();

  for (const item of items) {
    if (!item.runId) continue;
    const status = terminalStatus(item);
    if (status) terminalByRunId.set(item.runId, item as FinalMessageItem | TerminalNoticeItem);
  }

  if (terminalByRunId.size === 0) return groupToolItems(items);

  for (const item of items) {
    if (!item.runId) continue;
    const terminal = terminalByRunId.get(item.runId);
    if (!terminal || item.id === terminal.id || item.type === 'user_turn') continue;
    const existing = processItemsByRunId.get(item.runId) ?? [];
    existing.push(item);
    processItemsByRunId.set(item.runId, existing);
  }

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
    const terminal = item.runId ? terminalByRunId.get(item.runId) : undefined;
    if (item.type === 'user_turn') {
      flush();
      result.push({ kind: 'item', item });
      continue;
    }
    if (terminal && item.id !== terminal.id) {
      continue;
    }
    if (terminal && item.id === terminal.id) {
      const runId = item.runId;
      if (!runId) continue;
      flush();
      result.push({
        kind: 'run_summary',
        id: `run_summary:${runId}`,
        runId,
        status: terminalStatus(item) ?? 'failed',
        terminalItem: terminal,
        processEntries: groupToolItems(processItemsByRunId.get(runId) ?? []),
      });
      continue;
    }
    if (item.type !== 'tool' || !canGroupTool(item)) {
      flush();
      result.push({ kind: 'item', item });
      continue;
    }
    const first = pending[0];
    if (first && !sameToolGroup(first, item)) {
      flush();
    }
    pending.push(item);
  }
  flush();
  return result;
}
