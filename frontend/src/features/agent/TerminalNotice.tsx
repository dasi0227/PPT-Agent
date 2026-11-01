import type { TerminalNoticeItem } from './eventReducer';
import { FinalChangeSummary, FinalMessage } from './FinalMessage';
import { MarkdownMessage } from './MarkdownMessage';

export function TerminalNotice({ item }: { item: TerminalNoticeItem }) {
  const message = item.reason === 'superseded' ? '此前任务因服务中断而结束。'
    : item.error?.code === 'PROVIDER_UNAVAILABLE' ? '模型服务请求失败。' : item.message;
  if (item.reply) {
    return <FinalMessage item={{
      id: item.id, type: 'final', runId: item.runId, timestamp: item.timestamp,
      messageId: item.reply.messageId, text: item.reply.text,
      affectedTargets: item.affectedTargets, durationMs: item.durationMs,
    }} />;
  }
  return <article className="pb-4 pt-2 text-sm leading-[1.65] text-text-900">
    <MarkdownMessage content={message} className="[&>p:first-child]:mt-0 [&>p:last-child]:mb-0" />
    {item.affectedTargets.length > 0 && <div className="mt-3"><FinalChangeSummary targets={item.affectedTargets} /></div>}
  </article>;
}
