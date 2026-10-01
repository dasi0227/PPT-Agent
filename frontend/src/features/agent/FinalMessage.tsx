import React from 'react';
import { ChevronDown, ChevronRight, Sparkle } from 'lucide-react';
import type { PublicTarget } from '../../api/types';
import { cn } from '../../lib/utils';
import type { FinalMessageItem } from './eventReducer';
import { MarkdownMessage } from './MarkdownMessage';
import { MessageMetaActions } from './MessageMetaActions';
import { ChangeDiffCard } from './ChangeDiffCard';
import { ChangeStats } from './ChangeStats';
import { partLabel } from '../viewer/semanticLabels';

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
  return `${targets.length} 项内容已更改`;
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

function FinalSourceEntry({ target }: { target: PublicTarget }) {
  const [expanded, setExpanded] = React.useState(false);
  return <div className="border-b border-border last:border-b-0">
    <button type="button" aria-expanded={expanded} onClick={() => setExpanded(value => !value)} className="ui-interactive flex min-h-10 w-full items-center gap-2 px-3 py-2 text-left text-[13px] text-text-900">
      <span className="min-w-0 flex-1 truncate">{targetLabel(target)}</span>
      {(target.insertions || target.deletions) ? <ChangeStats insertions={target.insertions} deletions={target.deletions} /> : null}
      {expanded ? <ChevronDown className="h-3.5 w-3.5 text-text-400" /> : <ChevronRight className="h-3.5 w-3.5 text-text-400" />}
    </button>
    {expanded && <div className="px-3 pb-3"><ChangeDiffCard target={target} /></div>}
  </div>;
}

export function FinalChangeSummary({ targets }: { targets: PublicTarget[] }) {
  const [expanded, setExpanded] = React.useState(false);
  const changes = orderedTargets(targets);

  return (
    <section className="mb-3 overflow-hidden rounded-[12px] border border-border bg-surface">
      <button
        type="button"
        aria-expanded={expanded}
        onClick={() => setExpanded((value) => !value)}
        className={cn(
          'ui-interactive grid min-h-10 w-full grid-cols-[24px_minmax(0,1fr)_auto] items-center gap-2 px-3 py-2 text-left text-[13px] text-text-900',
          expanded && 'border-b border-border',
        )}
      >
        <Sparkle className="h-4 w-4 text-success" strokeWidth={1.75} />
        <span className="truncate text-sm font-normal">{summaryText(changes)}</span>
        {expanded
          ? <ChevronDown className="h-3.5 w-3.5 text-text-400" strokeWidth={1.75} />
          : <ChevronRight className="h-3.5 w-3.5 text-text-400" strokeWidth={1.75} />}
      </button>
      {expanded && (
        <div>
          {changes.map((target) => (
            <FinalSourceEntry key={targetKey(target)} target={target} />
          ))}
        </div>
      )}
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
