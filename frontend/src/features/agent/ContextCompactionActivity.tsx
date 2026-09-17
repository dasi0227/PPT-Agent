import { ChevronRight, Gauge } from 'lucide-react';
import { useId, useState } from 'react';
import { cn } from '../../lib/utils';
import type { ContextCompactionTimelineItem } from './eventReducer';
import { MarkdownMessage } from './MarkdownMessage';
import { TimelineDisclosure } from './TimelineDisclosure';

function formatDate(timestamp: number): string {
  const date = new Date(timestamp);
  const pad = (value: number) => String(value).padStart(2, '0');
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

function formatReclaimedTokens(tokens: number): string {
  return `${(Math.max(0, tokens) / 1000).toFixed(1)}k`;
}

function Divider() {
  return <span className="h-2.5 w-px shrink-0 bg-border" aria-hidden="true" />;
}

export function ContextCompactionActivity({ item }: { item: ContextCompactionTimelineItem }) {
  const [expanded, setExpanded] = useState(false);
  const metadataId = useId();
  const before = item.maxTokens > 0 ? Math.round(item.beforeTokens / item.maxTokens * 100) : 0;
  const after = item.maxTokens > 0 ? Math.round(item.afterTokens / item.maxTokens * 100) : 0;

  return (
    <div className="overflow-hidden">
      <button
        type="button"
        aria-expanded={expanded}
        aria-label={`compact: ${item.title}`}
        aria-describedby={metadataId}
        onClick={() => setExpanded((value) => !value)}
        className="grid min-h-10 w-full grid-cols-[26px_minmax(0,1fr)_14px] items-start gap-2 rounded-sm px-1 py-1.5 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
      >
        <span className="grid h-[26px] w-[26px] place-items-center rounded-full border border-success/35 bg-success-soft text-success">
          <Gauge className="h-3.5 w-3.5" strokeWidth={1.8} />
        </span>
        <span className="min-w-0">
          <span className="block truncate text-xs font-semibold">
            <span className="text-text-400">compact: </span>
            <span className="text-text-900">{item.title}</span>
          </span>
          <span id={metadataId} className="mt-0.5 flex min-w-0 flex-wrap items-center gap-x-1.5 gap-y-0.5 font-mono text-[10px] text-text-400">
            <span>{formatDate(item.timestamp)}</span>
            <Divider />
            <span>{item.trigger === 'auto' ? '自动触发' : '手动触发'}</span>
            <Divider />
            <span className="inline-flex whitespace-nowrap">
              窗口 {before}% → <span className="text-success">{after}%</span>&nbsp;回收&nbsp;
              <span className="text-danger">{formatReclaimedTokens(item.reclaimedTokens)}</span>&nbsp;Token
            </span>
          </span>
        </span>
        <ChevronRight
          className={cn(
            'mt-1 h-3.5 w-3.5 text-text-400 transition-transform motion-reduce:transition-none',
            expanded && 'rotate-90',
          )}
          strokeWidth={1.75}
        />
      </button>
      <TimelineDisclosure open={expanded}>
        {expanded && (
          <div className="ml-[34px] border-t border-border py-2 pr-2 text-xs leading-[1.7] text-text-600">
            <MarkdownMessage content={item.summary} />
          </div>
        )}
      </TimelineDisclosure>
    </div>
  );
}
