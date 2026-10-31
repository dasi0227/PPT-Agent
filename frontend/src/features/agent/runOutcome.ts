import type { Run } from '../../api/types';
import { readPublicEvent } from '../../api/sse';
import { reduceSSEEvent, settleRunActivities, type TimelineItem, type TerminalNoticeItem } from './eventReducer';

export function runStopped(status: Run['status']): boolean {
  return ['paused', 'done', 'failed', 'canceled'].includes(status);
}

const pauseMessages: Record<string, string> = {
  server_shutdown: '服务已关闭，本次任务未能完成。',
  server_restarted: '服务已重启，本次任务未能完成。',
  journal_write_failed: '运行记录保存失败，本次任务已停止。',
  transcript_write_failed: '对话记录保存失败，本次任务已停止。',
  checkpoint_write_failed: '任务进度保存失败，本次任务已停止。',
  input_decision_write_failed: '输入处理失败，本次任务已停止。',
  input_reply_write_failed: '回复保存失败，本次任务已停止。',
  approval_commit_failed: '审批结果保存失败，本次任务已停止。',
  scope_commit_failed: '执行范围保存失败，本次任务已停止。',
  event_protocol_invalid: '运行事件异常，本次任务已停止。',
  resume_event_failed: '继续执行时发生异常，本次任务未能恢复。',
};

// Snapshots reconcile missing events; they never replace confirmed tool outcomes.
export function reconcileRunTimeline(items: TimelineItem[], run: Run): TimelineItem[] {
  if (!runStopped(run.status)) return items;
  if (run.status !== 'paused') items = items.filter(item => !(item.type === 'terminal_notice' && item.runId === run.id && item.pauseReason));
  for (const result of run.recorded_tool_results ?? []) {
    const existing = items.find(item => item.type === 'tool' && item.runId === run.id && item.callId === result.call_id);
    if (existing?.type === 'tool' && !['running', 'interrupted', 'canceled'].includes(existing.status)) continue;
    try {
      const event = readPublicEvent('tool.completed', result, undefined, run.id);
      if (event) {
        items = reduceSSEEvent(items, event);
        if (!existing) {
          const added = items[items.length - 1];
          const next = items.slice(0, -1);
          let index = -1;
          next.forEach((item, i) => { if (item.runId === run.id) index = i; });
          if (next[index]?.type === 'terminal_notice' || next[index]?.type === 'final') index--;
          next.splice(index + 1, 0, added);
          items = next;
        }
      }
    } catch { /* An invalid projection cannot establish a completed outcome. */ }
  }
  const terminal = items.find((item): item is TerminalNoticeItem => item.type === 'terminal_notice' && item.runId === run.id);
  items = settleRunActivities(items, run.id, terminal?.reason === 'user_requested' ? 'canceled' : 'interrupted');
  if (terminal && !terminal.pauseReason) return items;
  if (run.status === 'done' && items.some(item => item.type === 'final' && item.runId === run.id)) return items;
  const timestamp = (run.paused_at || run.updated_at || 0) * 1000
    || [...items].reverse().find(item => item.runId === run.id)?.timestamp || Date.now();
  if (run.status === 'done') return items; // Do not invent a successful final reply.
  const reviewSaved = run.recorded_tool_results?.some(result => result.tool === 'review_ppt' && result.status === 'completed');
  const notice: TerminalNoticeItem = {
    id: `${run.id}:terminal`, type: 'terminal_notice', runId: run.id,
    status: run.status === 'canceled' ? 'canceled' : 'error', timestamp, affectedTargets: [], durationMs: run.active_duration_ms,
    ...(run.status === 'paused' ? { pauseReason: run.pause_reason || 'interrupted' } : {}),
    message: run.status === 'paused'
      ? run.pause_reason === 'event_protocol_invalid' && reviewSaved ? '审查结果发布异常，任务已停止。'
        : pauseMessages[run.pause_reason ?? ''] ?? '服务中断，本次任务未能完成。'
      : run.status === 'canceled' ? '任务已停止。' : '任务执行时发生异常，未能完成。',
  };
  // Insert beside its own run, including when an older run is reconciled later.
  const next = items.filter(item => !(item.type === 'terminal_notice' && item.runId === run.id));
  let index = -1;
  next.forEach((item, i) => { if (item.runId === run.id) index = i; });
  next.splice(index + 1, 0, notice);
  return next;
}

export function settleTimelineActivities(items: TimelineItem[]): TimelineItem[] {
  for (const item of items) {
    if (!item.runId || (item.type !== 'terminal_notice' && item.type !== 'final')) continue;
    items = settleRunActivities(items, item.runId,
      item.type === 'terminal_notice' && item.reason === 'user_requested' ? 'canceled' : 'interrupted');
  }
  return items;
}
