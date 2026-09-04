import { CheckCircle2, ChevronDown, ChevronUp } from 'lucide-react';
import { useLayoutEffect, useRef, useState } from 'react';
import type { ContextCompactionTimelineItem } from './eventReducer';
import { MarkdownMessage } from './MarkdownMessage';

const COLLAPSED_HEIGHT = 300;

function formatDuration(durationMs: number): string {
  return `${(Math.max(0, durationMs) / 1000).toFixed(1)}s`;
}

function formatTokens(tokens: number): string {
  const absolute = Math.abs(tokens);
  if (absolute >= 1000) return `${(absolute / 1000).toFixed(1)}k`;
  return absolute.toLocaleString();
}

export function ContextCompactionActivity({ item }: { item: ContextCompactionTimelineItem }) {
  const [expanded, setExpanded] = useState(false);
  const [overflowing, setOverflowing] = useState(false);
  const summaryRef = useRef<HTMLDivElement>(null);
  const before = item.maxTokens > 0 ? Math.round(item.beforeTokens / item.maxTokens * 100) : 0;
  const after = item.maxTokens > 0 ? Math.round(item.afterTokens / item.maxTokens * 100) : 0;

  useLayoutEffect(() => {
    const summary = summaryRef.current;
    if (!summary) return undefined;
    const measure = () => setOverflowing(summary.scrollHeight > COLLAPSED_HEIGHT + 1);
    measure();
    const observer = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(measure);
    observer?.observe(summary);
    return () => observer?.disconnect();
  }, [item.summary]);

  return (
    <article className="context-compaction-card overflow-hidden rounded-lg border border-border bg-surface">
      <header className="flex min-h-10 items-center gap-2 border-b border-border px-3 py-2">
        <span className="inline-flex items-center gap-1.5 rounded-md bg-success-soft px-2 py-1 text-[11px] font-semibold text-success">
          <CheckCircle2 className="h-3.5 w-3.5" strokeWidth={1.75} />
          上下文已压缩
        </span>
        <span className="text-[10px] text-text-400">
          {item.trigger === 'auto' ? '自动触发' : '手动触发'} · 耗时 {formatDuration(item.durationMs)}
        </span>
      </header>

      <div className="grid grid-cols-2 gap-2 px-3 py-3">
        <div className="rounded-md border border-border bg-panel px-2.5 py-2">
          <div className="text-[10px] font-semibold text-text-400">窗口占用</div>
          <div className="mt-0.5 flex items-baseline gap-1.5 font-mono text-sm font-bold text-text-900">
            {before}%
            <span className="text-[10px] text-text-400">→</span>
            <span className="text-success">{after}%</span>
          </div>
        </div>
        <div className="rounded-md border border-border bg-panel px-2.5 py-2">
          <div className="text-[10px] font-semibold text-text-400">回收 Token</div>
          <div className="mt-0.5 font-mono text-sm font-bold text-success">-{formatTokens(item.reclaimedTokens)}</div>
        </div>
      </div>

      <div className="px-3 pb-3">
        <h3 className="mb-1.5 text-[11px] font-semibold text-text-600">压缩摘要</h3>
        <div className="relative">
          <div
            ref={summaryRef}
            className="rounded-md border border-border bg-panel px-3 py-2.5 text-xs leading-[1.7] text-text-600"
            style={expanded ? undefined : { maxHeight: COLLAPSED_HEIGHT, overflow: 'hidden' }}
          >
            <MarkdownMessage content={item.summary} />
          </div>
          {overflowing && !expanded && (
            <div className="pointer-events-none absolute inset-x-px bottom-px h-16 rounded-b-md bg-gradient-to-b from-transparent to-panel" />
          )}
          {overflowing && (
            <button
              type="button"
              onClick={() => setExpanded((value) => !value)}
              className="absolute bottom-2 left-1/2 inline-flex h-7 -translate-x-1/2 items-center gap-1 rounded-full border border-border-strong bg-surface px-3 text-[11px] font-semibold text-text-600 shadow-sm hover:text-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
              aria-expanded={expanded}
            >
              {expanded ? <ChevronUp className="h-3 w-3" /> : <ChevronDown className="h-3 w-3" />}
              {expanded ? '收起' : '展开全文'}
            </button>
          )}
        </div>
      </div>
    </article>
  );
}
