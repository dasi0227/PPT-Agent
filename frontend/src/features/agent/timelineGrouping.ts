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

function terminalStatus(item: TimelineItem): 'completed' | 'failed' | 'error' | 'canceled' | null {
  if (item.type === 'final') return 'completed';
  if (item.type === 'terminal_notice') return item.status;
  return null;
}

export function visibleTimelineItems(items: TimelineItem[], showToolFailures: boolean): TimelineItem[] {
  if (showToolFailures) return items;
  return items.filter(item => item.type !== 'tool' || !['failed', 'blocked', 'interrupted', 'canceled'].includes(item.status));
}

// Display a handled input as a normal reply without changing the persisted
// cancellation that prevents rejected instructions from resuming execution.
function projectInputReplies(items: TimelineItem[], context: TimelineItem[]): TimelineItem[] {
  const replacements = new Map<string, { terminal: TimelineItem; reply?: TimelineItem; before?: string }>();
  const anchored = new Map<string, TimelineItem>();
  for (const item of items) {
    if (item.type !== 'terminal_notice') continue;
    if (item.reason === 'input_detection_failed') {
      replacements.set(item.id, { terminal: { ...item, status: 'error' } });
      continue;
    }
    if (item.reason !== 'input_rejected' || !item.reply) continue;
    const inputReply = item.reply;
    const trigger = items.find(candidate => candidate.runId === item.runId && candidate.type === 'user_turn'
      && candidate.clientMessageId && `input_${candidate.clientMessageId}` === inputReply.messageId);
    const hasOriginalInput = context.some(candidate => candidate.runId === item.runId
      && candidate.type === 'user_turn' && !candidate.clientMessageId);
    const hasTask = !hasOriginalInput || context.some(candidate => candidate.runId === item.runId
      && (candidate.type === 'user_turn' ? Boolean(candidate.clientMessageId)
        : !['input_reply', 'terminal_notice'].includes(candidate.type)))
      || item.affectedTargets.length > 0;
    if (!hasTask) {
      replacements.set(item.id, { terminal: {
        id: item.id, type: 'final', runId: item.runId, timestamp: item.timestamp,
        messageId: inputReply.messageId, text: inputReply.text,
        affectedTargets: item.affectedTargets, durationMs: item.durationMs,
      } });
      continue;
    }
    const terminal: TerminalNoticeItem = { ...item, reply: undefined };
    replacements.set(item.id, { terminal, before: trigger?.id, reply: {
      id: `${item.id}:reply`, type: 'input_reply', runId: item.runId, timestamp: item.timestamp,
      messageId: inputReply.messageId, text: inputReply.text, affectedTargets: [],
    } });
    if (trigger) anchored.set(trigger.id, terminal);
  }
  return items.flatMap(item => {
    const before = anchored.get(item.id);
    const replacement = replacements.get(item.id);
    return [
      ...(before ? [before] : []),
      ...(replacement ? [
        ...(!replacement.before ? [replacement.terminal] : []),
        ...(replacement.reply ? [replacement.reply] : []),
      ] : [item]),
    ];
  });
}

export function groupTimelineItems(visibleItems: TimelineItem[], context: TimelineItem[] = visibleItems): DisplayEntry[] {
  // Use the complete context so hiding failed tools cannot turn a stopped
  // authoring task into an apparently successful fixed reply.
  const items = projectInputReplies(visibleItems, context);
  const terminalByRunId = new Map<string, FinalMessageItem | TerminalNoticeItem>();
  const processItemsByRunId = new Map<string, TimelineItem[]>();

  for (const item of items) {
    if (!item.runId) continue;
    const status = terminalStatus(item);
    if (status) {
      // A stop remains authoritative over an earlier drafted final, including
      // when the stop summary is moved ahead of its rejected steering input.
      if (terminalByRunId.get(item.runId)?.type === 'terminal_notice' && item.type === 'final') continue;
      terminalByRunId.set(item.runId, item as FinalMessageItem | TerminalNoticeItem);
    }
  }

  if (terminalByRunId.size === 0) return groupToolItems(items);

  for (const item of items) {
    if (!item.runId) continue;
    const terminal = terminalByRunId.get(item.runId);
    if (!terminal || item.id === terminal.id || item.type === 'user_turn' || item.type === 'input_reply') continue;
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
    if (item.type === 'user_turn' || item.type === 'input_reply') {
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
