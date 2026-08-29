import React from 'react';
import { ChevronDown, ChevronRight, Crosshair, ExternalLink, Sparkle } from 'lucide-react';
import type { PublicTarget } from '../../api/types';
import { useDeckStore } from '../../stores/deckStore';
import { useProjectStore } from '../../stores/projectStore';
import { orderedSlides } from '../deck/selectors';
import { cn } from '../../lib/utils';
import type { FinalMessageItem } from './eventReducer';
import { MarkdownMessage } from './MarkdownMessage';
import { MessageMetaActions } from './MessageMetaActions';

function targetKey(target: PublicTarget): string {
  return `${target.type}:${target.slide_id ?? ''}:${target.part}`;
}

function targetLabel(target: PublicTarget): string {
  if (target.type === 'deck') {
    if (target.part === 'manifest') return '演示内容';
    if (target.part === 'outline') return '目录结构';
    if (target.part === 'design') return '视觉设计';
  }
  const page = target.display_name || '页面';
  if (target.part === 'spec') return `${page}设计稿`;
  if (target.part === 'html') return `${page}幻灯片`;
  return page;
}

function summaryText(targets: PublicTarget[]): string {
  return `${targets.length} 个文件已更改`;
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
  spec: 3,
  html: 4,
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

export function FinalChangeSummary({ targets }: { targets: PublicTarget[] }) {
  const [expanded, setExpanded] = React.useState(false);
  const snapshot = useProjectStore((state) => state.activeProjectId ? state.contentByProjectId[state.activeProjectId] : undefined);
  const slides = orderedSlides(snapshot);
  const setCurrentSlideId = useDeckStore((state) => state.setCurrentSlideId);
  const setGlobalView = useDeckStore((state) => state.setGlobalView);
  const changes = orderedTargets(targets, slides.map((slide) => slide.id));
  const insertions = sumStat(changes, 'insertions');
  const deletions = sumStat(changes, 'deletions');

  const jumpToTarget = (target: PublicTarget) => {
    if (target.type === 'slide' && target.slide_id) {
      if (slides.some((slide) => slide.id === target.slide_id)) setCurrentSlideId(target.slide_id);
      setGlobalView(target.part === 'html' ? 'html' : 'outline');
      return;
    }
    setGlobalView('outline');
  };

  return (
    <section className="mb-3 overflow-hidden rounded-[12px] border border-border bg-surface">
      <button
        type="button"
        aria-expanded={expanded}
        onClick={() => setExpanded((value) => !value)}
        className={cn(
          'grid min-h-10 w-full grid-cols-[24px_minmax(0,1fr)_auto_auto] items-center gap-2 px-3 py-2 text-left text-[13px] text-text-900',
          expanded && 'border-b border-border',
        )}
      >
        <Sparkle className="h-4 w-4 text-success" strokeWidth={1.75} />
        <span className="truncate text-sm font-semibold">{summaryText(changes)}</span>
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
            <div
              key={targetKey(target)}
              className="grid min-h-10 grid-cols-[24px_minmax(0,1fr)_auto_auto] items-center gap-2 border-b border-border px-3 py-2 text-[13px] text-text-900 last:border-b-0"
            >
              <Sparkle className="h-4 w-4 text-success" strokeWidth={1.75} />
              <span className="truncate font-medium">{targetLabel(target)}</span>
              {(target.insertions || target.deletions) ? (
                <span className="font-mono text-xs">
                  <span className="text-success">+{target.insertions ?? 0}</span>{' '}
                  <span className="text-danger">-{target.deletions ?? 0}</span>
                </span>
              ) : <span />}
              <span className="flex items-center gap-1">
                <button
                  type="button"
                  aria-label={`跳转到${targetLabel(target)}`}
                  onClick={() => jumpToTarget(target)}
                  className="inline-flex h-6 w-6 items-center justify-center rounded-md border border-border text-text-600 hover:text-text-900"
                >
                  <Crosshair className="h-3.5 w-3.5" strokeWidth={1.75} />
                </button>
                {target.open_url ? (
                  <a
                    href={target.open_url}
                    aria-label={`打开${targetLabel(target)}文件`}
                    title={target.local_path ?? `打开${targetLabel(target)}文件`}
                    className="inline-flex h-6 w-6 items-center justify-center rounded-md border border-border text-text-600 hover:text-text-900"
                  >
                    <ExternalLink className="h-3.5 w-3.5" strokeWidth={1.75} />
                  </a>
                ) : (
                  <button
                    type="button"
                    aria-label={`打开${targetLabel(target)}文件`}
                    title="当前事件未提供可打开的本机文件链接"
                    disabled
                    className="inline-flex h-6 w-6 cursor-not-allowed items-center justify-center rounded-md border border-border text-text-400 opacity-50"
                  >
                    <ExternalLink className="h-3.5 w-3.5" strokeWidth={1.75} />
                  </button>
                )}
              </span>
            </div>
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
