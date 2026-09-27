import React from 'react';
import { ChevronDown, ChevronRight, Sparkle } from 'lucide-react';
import type { PublicTarget } from '../../api/types';
import { useProjectStore } from '../../stores/projectStore';
import { orderedSlides } from '../deck/selectors';
import { cn } from '../../lib/utils';
import type { FinalMessageItem } from './eventReducer';
import { MarkdownMessage } from './MarkdownMessage';
import { MessageMetaActions } from './MessageMetaActions';
import { TargetSourceCard } from './SourceCard';
import { partLabel } from '../viewer/semanticLabels';

function targetKey(target: PublicTarget): string {
  return `${target.type}:${target.slide_id ?? ''}:${target.part}`;
}

function targetLabel(target: PublicTarget): string {
  if (target.type === 'deck') return partLabel(target.part);
  const page = target.display_name || '页面';
  if (target.part === 'spec') return `${page}${partLabel('spec')}`;
  if (target.part === 'html') return `${page}幻灯片`;
  return page;
}

function summaryText(targets: PublicTarget[]): string {
  return `${targets.length} 项内容已更改`;
}

function sumStat(targets: PublicTarget[], key: 'insertions' | 'deletions'): number {
  return targets.reduce((total, target) => total + (target[key] ?? 0), 0);
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
  manifest: 0,
  outline: 1,
  design: 2,
  content: 3,
  spec: 4,
  html: 5,
};

function orderedTargets(targets: PublicTarget[], slideIds: string[]): PublicTarget[] {
  const ordinalById = new Map(slideIds.map((id, index) => [id, index]));
  return [...uniqueTargets(targets)].sort((left, right) => {
    const group = targetGroupRank[left.part] - targetGroupRank[right.part];
    if (group !== 0) return group;
    const leftOrdinal = left.slide_id ? (ordinalById.get(left.slide_id) ?? Number.MAX_SAFE_INTEGER) : -1;
    const rightOrdinal = right.slide_id ? (ordinalById.get(right.slide_id) ?? Number.MAX_SAFE_INTEGER) : -1;
    if (leftOrdinal !== rightOrdinal) return leftOrdinal - rightOrdinal;
    return targetKey(left).localeCompare(targetKey(right));
  });
}

function FinalSourceEntry({ target }: { target: PublicTarget }) {
  const [expanded, setExpanded] = React.useState(false);
  return <div className="border-b border-border last:border-b-0">
    <button type="button" aria-expanded={expanded} onClick={() => setExpanded(value => !value)} className="ui-interactive flex min-h-10 w-full items-center gap-2 px-3 py-2 text-left text-[13px] text-text-900">
      <span className="min-w-0 flex-1 truncate">{targetLabel(target)}</span>
      {(target.insertions || target.deletions) ? <span className="font-mono text-xs"><span className="text-success">+{target.insertions ?? 0}</span>{' '}<span className="text-danger">-{target.deletions ?? 0}</span></span> : null}
      {expanded ? <ChevronDown className="h-3.5 w-3.5 text-text-400" /> : <ChevronRight className="h-3.5 w-3.5 text-text-400" />}
    </button>
    {expanded && <div className="px-3 pb-3"><TargetSourceCard target={target} /></div>}
  </div>;
}

export function FinalChangeSummary({ targets }: { targets: PublicTarget[] }) {
  const [expanded, setExpanded] = React.useState(false);
  const snapshot = useProjectStore((state) => state.activeProjectId ? state.contentByProjectId[state.activeProjectId] : undefined);
  const slides = orderedSlides(snapshot);
  const changes = orderedTargets(targets, slides.map((slide) => slide.id));
  const insertions = sumStat(changes, 'insertions');
  const deletions = sumStat(changes, 'deletions');


  return (
    <section className="mb-3 overflow-hidden rounded-[12px] border border-border bg-surface">
      <button
        type="button"
        aria-expanded={expanded}
        onClick={() => setExpanded((value) => !value)}
        className={cn(
          'ui-interactive grid min-h-10 w-full grid-cols-[24px_minmax(0,1fr)_auto_auto] items-center gap-2 px-3 py-2 text-left text-[13px] text-text-900',
          expanded && 'border-b border-border',
        )}
      >
        <Sparkle className="h-4 w-4 text-success" strokeWidth={1.75} />
        <span className="truncate text-sm font-normal">{summaryText(changes)}</span>
        {(insertions > 0 || deletions > 0) && (
          <span className="font-mono text-xs">
            <span className="text-success">+{insertions}</span>{' '}
            <span className="text-danger">-{deletions}</span>
          </span>
        )}
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
