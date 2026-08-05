import type { FinalMessageItem, TerminalNoticeItem, TimelineItem, ToolActivityItem } from './eventReducer';

export type DisplayEntry =
  | { kind: 'item'; item: TimelineItem }
  | { kind: 'tool_group'; id: string; items: ToolActivityItem[] }
  | {
      kind: 'run_summary';
      id: string;
      runId: string;
      status: 'completed' | 'failed' | 'canceled';
      terminalItem: FinalMessageItem | TerminalNoticeItem;
      processEntries: DisplayEntry[];
    };

function canGroupTool(item: ToolActivityItem, currentSlideId?: string): boolean {
  return item.status === 'completed'
    && !item.error
    && (item.preview?.warnings.length ?? 0) === 0
    // 仅当条目确有 slide_id 且正是当前查看页时才排除；deck 级（无 slide_id）始终可汇聚。
    && !(item.target?.slide_id !== undefined && item.target.slide_id === currentSlideId);
}

function groupToolItems(items: TimelineItem[], currentSlideId?: string): DisplayEntry[] {
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

function terminalStatus(item: TimelineItem): 'completed' | 'failed' | 'canceled' | null {
  if (item.type === 'final') return 'completed';
  if (item.type === 'terminal_notice') return item.status;
  return null;
}

export function groupTimelineItems(items: TimelineItem[], currentSlideId?: string): DisplayEntry[] {
  const terminalByRunId = new Map<string, FinalMessageItem | TerminalNoticeItem>();
  const processItemsByRunId = new Map<string, TimelineItem[]>();

  for (const item of items) {
    if (!item.runId) continue;
    const status = terminalStatus(item);
    if (status) terminalByRunId.set(item.runId, item as FinalMessageItem | TerminalNoticeItem);
  }

  if (terminalByRunId.size === 0) return groupToolItems(items, currentSlideId);

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
    if (pending.length >= 3) {
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
        processEntries: groupToolItems(processItemsByRunId.get(runId) ?? [], currentSlideId),
      });
      continue;
    }
    if (item.type !== 'tool' || !canGroupTool(item, currentSlideId)) {
      flush();
      result.push({ kind: 'item', item });
      continue;
    }
    const first = pending[0];
    if (first && first.tool !== item.tool) {
      flush();
    }
    pending.push(item);
  }
  flush();
  return result;
}
