import React from 'react';
import { FileDiff } from 'lucide-react';
import type { PublicTarget } from '../../api/types';
import type { FinalMessageItem } from './eventReducer';
import { MarkdownMessage } from './MarkdownMessage';
import { MessageMetaActions } from './MessageMetaActions';
import { ChangeDiffCard, useChangePreviewEnabled } from './ChangeDiffCard';
import { openSourceTarget } from './SourceCard';
import { ChangeStats } from './ChangeStats';
import { partLabel } from '../viewer/semanticLabels';
import { TimelineDisclosure, TimelineChevron } from './TimelineDisclosure';

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
  manifest: 1,
  design: 2,
  spec: 3,
  html: 4,
  content: 5,
};

function orderedTargets(targets: PublicTarget[]): PublicTarget[] {
  // Backend order is frozen at the terminal boundary, including deleted pages.
  return uniqueTargets(targets).sort((left, right) => targetGroupRank[left.part] - targetGroupRank[right.part]);
}

function FinalSourceEntry({ target, expanded, onToggle }: {
  target: PublicTarget; expanded: boolean; onToggle: () => void;
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
      <ChangeDiffCard target={target} variant="inline" />
    </TimelineDisclosure>
  </div>;
}

export function FinalChangeSummary({ targets }: { targets: PublicTarget[] }) {
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
        <div>
          {changes.map((target) => (
            <FinalSourceEntry key={targetKey(target)} target={target} expanded={expandedTarget === targetKey(target)}
              onToggle={() => setExpandedTarget(current => current === targetKey(target) ? null : targetKey(target))} />
          ))}
        </div>
      </TimelineDisclosure>
    </section>
  );
}

export const FinalMessage: React.FC<{ item: FinalMessageItem }> = ({ item }) => {
  return (
    <article className="group pb-4 pt-2 text-sm leading-[1.65] text-text-900">
      <MarkdownMessage content={item.text} />
      {item.affectedTargets.length > 0 && <FinalChangeSummary targets={item.affectedTargets} />}
      <MessageMetaActions text={item.text} timestamp={item.timestamp} label="复制回复" />
    </article>
  );
};
