import { useId, useState, type ReactNode } from 'react';
import { Gauge, GitCommitHorizontal, Loader2, Signature } from 'lucide-react';
import { cn } from '../../lib/utils';
import type { CommandTimelineItem, ContextCompactionTimelineItem, GitCommitTimelineItem } from './eventReducer';
import { commandSteps } from './CommandActivity';
import { LongContent } from './LongContent';
import { MarkdownMessage } from './MarkdownMessage';
import { TimelineDisclosure, TimelineChevron } from './TimelineDisclosure';

type AgentCommandItem = CommandTimelineItem | ContextCompactionTimelineItem | GitCommitTimelineItem;

// Runtime commands share the tool row, while user-triggered commands retain
// their independent CommandActivity presentation.
export function AgentCommandRow({ item }: { item: AgentCommandItem }) {
  const [open, setOpen] = useState(false);
  const detailId = useId();
  const kind = item.type === 'git_commit' ? 'commit' : item.type === 'context_compaction' ? 'compact' : item.kind;
  const status = item.type === 'context_compaction' ? 'completed' : item.status;
  const running = status === 'loading';
  const names = { commit: '提交项目版本', compact: '压缩上下文', rename: '命名会话', polish: '润色输入内容' };
  const Icon = kind === 'commit' ? GitCommitHorizontal : kind === 'compact' ? Gauge : Signature;
  const label = running ? `正在${names[kind]}`
    : status === 'failed' ? `${names[kind]}失败`
      : status === 'canceled' ? `${names[kind]}已停止`
        : kind === 'compact' ? `已压缩上下文：${item.title}`
          : kind === 'rename' && item.type === 'command' ? item.content || item.title
          : item.title || names[kind];
  let metadata: ReactNode = undefined;
  let body: ReactNode;
  if (running) {
    const phase = item.type === 'context_compaction' ? -1 : item.phase ?? -1;
    body = <p role="status">{commandSteps[kind][phase] ?? '正在准备…'}</p>;
  } else if (status !== 'completed') {
    body = <p className={status === 'failed' ? 'text-danger' : 'text-text-600'}>{item.commandError || label}</p>;
  } else if (item.type === 'git_commit') {
    metadata = item.hash ? <>
      <span>{item.branch} {item.hash}</span><span aria-hidden="true">·</span>
      <span>{item.filesChanged ?? 0} files</span><span aria-hidden="true">·</span>
      <span className="text-success">+{item.insertions ?? 0}</span>
      <span className="text-danger">−{item.deletions ?? 0}</span>
    </> : undefined;
    body = <>
      <p className="font-medium text-text-900">{item.title}</p>
      {!!item.items?.length && <ul className="mt-2 list-disc space-y-1 pl-5 marker:text-success">
        {item.items.map((text, index) => <li key={index}>{text}</li>)}
      </ul>}
    </>;
  } else if (item.type === 'context_compaction') {
    const percent = (tokens: number) => item.maxTokens > 0 ? Math.round(tokens / item.maxTokens * 100) : 0;
    metadata = <>
      <span>窗口 {percent(item.beforeTokens)}% → <span className="text-success">{percent(item.afterTokens)}%</span></span>
      <span aria-hidden="true">·</span>
      <span>回收 <span className="text-danger">{(Math.max(0, item.reclaimedTokens) / 1000).toFixed(1)}k</span> Token</span>
    </>;
    body = <MarkdownMessage content={item.content} />;
  } else {
    body = <p>{item.content || item.title}</p>;
  }
  return <div className="min-w-0 overflow-hidden rounded-lg" aria-busy={running}>
    <button type="button" aria-expanded={open} aria-controls={detailId} title={label}
      onClick={() => setOpen(value => !value)}
      className="timeline-disclosure-trigger ui-interactive grid min-h-8 w-full grid-cols-[16px_minmax(0,1fr)_16px] items-center gap-2 rounded-md bg-transparent px-1.5 py-1 text-left">
      {running ? <Loader2 className="h-4 w-4 animate-spin text-warning motion-reduce:animate-none" />
        : <Icon className={cn('h-4 w-4', status === 'failed' ? 'text-danger' : status === 'canceled' ? 'text-text-400' : 'text-success')} />}
      <span className="min-w-0 truncate text-[13px] font-normal text-text-900">{label}</span>
      <TimelineChevron open={open} />
    </button>
    <TimelineDisclosure open={open}>
      {open && <section id={detailId} className="timeline-detail-card overflow-hidden rounded-[10px] border border-border bg-surface [overflow-wrap:anywhere]">
        {metadata && <header className="mx-4 flex flex-wrap items-center gap-x-[7px] gap-y-1 border-b border-border py-[11px] text-[11px] leading-[18px] text-text-600 tabular-nums">{metadata}</header>}
        <LongContent className="command-result-content" contentClassName="text-[13px] leading-[1.85] text-text-700">
          {body}
        </LongContent>
      </section>}
    </TimelineDisclosure>
  </div>;
}
