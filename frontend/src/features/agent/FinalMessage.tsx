import React from 'react';
import { FileDiff, Loader2, RotateCcw } from 'lucide-react';
import type { ArtifactDiff, PublicTarget } from '../../api/types';
import { IconButton } from '../../components/ui/primitives';
import { useSessionStore } from '../../stores/sessionStore';
import type { FinalMessageItem } from './timelineTypes';
import { MarkdownMessage } from './MarkdownMessage';
import { MessageMetaActions } from './MessageMetaActions';
import { ChangeDiffCard, useChangePreviewEnabled } from './ChangeDiffCard';
import { openSourceTarget } from './SourceCard';
import { ChangeStats } from './ChangeStats';
import { partLabel } from '../viewer/semanticLabels';
import { TimelineDisclosure, TimelineChevron } from './TimelineDisclosure';
import { useActiveThreadId } from './useActiveSession';

function targetKey(target: PublicTarget): string {
  return `${target.type}:${target.slide_id ?? target.diff?.filename ?? target.display_name ?? ''}:${target.part}`;
}

function targetLabel(target: PublicTarget): string {
  if (target.type === 'file') return target.display_name || target.diff?.filename || '文件';
  if (target.type === 'deck') return partLabel(target.part);
  const page = target.display_name || '页面';
  if (target.part === 'spec') return `${page}设计稿`;
  if (target.part === 'html') return `${page}幻灯片`;
  return page;
}

function summaryText(targets: PublicTarget[]): string {
  return `已更改 ${targets.length} 项内容`;
}

function uniqueTargets(targets: PublicTarget[]): PublicTarget[] {
  const seen = new Set<string>();
  const out: PublicTarget[] = [];
  for (const target of targets) {
    const key = targetKey(target);
    if (seen.has(key)) continue;
    seen.add(key);
    out.push(target);
  }
  return out;
}

const targetGroupRank: Record<PublicTarget['part'], number> = {
  outline: 0,
  meta: 1,
  constraints: 2,
  spec: 3,
  html: 4,
  content: 5,
};

function orderedTargets(targets: PublicTarget[]): PublicTarget[] {
  // Backend order is frozen at the terminal boundary, including deleted pages.
  return uniqueTargets(targets).sort((left, right) => targetGroupRank[left.part] - targetGroupRank[right.part]);
}

function FinalChangeDiff({ target, itemId }: { target: PublicTarget; itemId: string }) {
  const threadId = useActiveThreadId();
  const ref = target.diff_ref;
  const ready = useSessionStore(state => threadId ? state.sessions[threadId]?.loadStatus === 'ready' : false);
  const diff = useSessionStore(state => threadId && ref ? state.sessions[threadId]?.detailsByRef[ref] : undefined);
  const detail = useSessionStore(state => threadId && ref ? state.sessions[threadId]?.detailStates[ref] : undefined);
  const load = () => { if (threadId && ref) void useSessionStore.getState().loadDetail(threadId, itemId, ref); };

  React.useEffect(() => {
    if (threadId && ref && ready && !diff && detail?.status !== 'loading' && !detail?.error) {
      void useSessionStore.getState().loadDetail(threadId, itemId, ref);
    }
  }, [threadId, itemId, ref, ready, diff, detail?.status, detail?.error]);

  if (ref && !diff) return <div className="border-t border-border bg-timeline-card px-3.5 py-3">
    {detail?.error ? <div role="alert" className="flex items-center gap-2 text-xs text-danger">
      <span className="min-w-0 flex-1 break-words">{detail.error.message}</span>
      <IconButton label="重试" onClick={load}><RotateCcw className="h-3.5 w-3.5" /></IconButton>
    </div> : <Loader2 role="status" aria-label="加载中" className="h-4 w-4 animate-spin text-text-400 motion-reduce:animate-none" />}
  </div>;
  return <ChangeDiffCard target={diff ? { ...target, diff: diff as unknown as ArtifactDiff } : target} variant="inline" />;
}

function FinalSourceEntry({ target, itemId, expanded, onToggle }: {
  target: PublicTarget; itemId: string; expanded: boolean; onToggle: () => void;
}) {
  const contentId = React.useId();
  const label = targetLabel(target);
  const canPreview = useChangePreviewEnabled(target);
  return <div className="border-t border-border">
    <div className="final-change-head ui-interactive flex min-h-[42px] items-center bg-surface text-[13px] text-text-900">
      <button type="button" disabled={!canPreview} onClick={() => openSourceTarget(target)}
        className="min-w-0 self-stretch py-2 pl-3.5 pr-2 text-left hover:underline focus-visible:underline focus-visible:outline-none disabled:cursor-default disabled:no-underline">
        <span className="block truncate">{label}</span>
      </button>
      <button type="button" aria-label={`${label}变更`} aria-expanded={expanded} aria-controls={contentId} onClick={onToggle}
        className="flex flex-1 self-stretch items-center justify-end py-2 pl-2 pr-3.5 focus-visible:outline-none">
        {(target.insertions || target.deletions) ? <ChangeStats insertions={target.insertions} deletions={target.deletions} /> : null}
      </button>
    </div>
    <TimelineDisclosure id={contentId} open={expanded} className="final-change-disclosure">
      {expanded && <FinalChangeDiff target={target} itemId={itemId} />}
    </TimelineDisclosure>
  </div>;
}

export function FinalChangeSummary({ targets, itemId }: { targets: PublicTarget[]; itemId: string }) {
  const [expanded, setExpanded] = React.useState(false);
  const [expandedTarget, setExpandedTarget] = React.useState<string | null>(null);
  const contentId = React.useId();
  const changes = orderedTargets(targets);

  return (
    <section className="mb-3 overflow-hidden rounded-[12px] border border-border bg-surface">
      <button
        type="button"
        aria-expanded={expanded}
        aria-controls={contentId}
        onClick={() => setExpanded((value) => !value)}
        className="timeline-disclosure-trigger ui-interactive grid min-h-10 w-full grid-cols-[24px_minmax(0,1fr)_auto] items-center gap-2 px-3 py-2 text-left text-[13px] text-text-900"
      >
        <FileDiff className="h-4 w-4 text-success" strokeWidth={1.75} />
        <span className="truncate text-sm font-normal">{summaryText(changes)}</span>
        <TimelineChevron open={expanded} />
      </button>
      <TimelineDisclosure id={contentId} open={expanded} className="final-change-disclosure">
        {expanded && <div>
          {changes.map((target) => (
            <FinalSourceEntry key={targetKey(target)} target={target} itemId={itemId} expanded={expandedTarget === targetKey(target)}
              onToggle={() => setExpandedTarget(current => current === targetKey(target) ? null : targetKey(target))} />
          ))}
        </div>}
      </TimelineDisclosure>
    </section>
  );
}

export const FinalMessage = React.memo(function FinalMessage({ item }: { item: FinalMessageItem }) {
  return (
    <article className="group pb-4 pt-2 text-sm leading-[1.65] text-text-900">
      <MarkdownMessage content={item.text} />
      {item.affectedTargets.length > 0 && <FinalChangeSummary targets={item.affectedTargets} itemId={item.id} />}
      <MessageMetaActions text={item.text} timestamp={item.timestamp} label="复制回复" />
    </article>
  );
});
