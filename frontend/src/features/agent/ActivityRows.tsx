import React, { useState } from 'react';
import {
  BrainCircuit,
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  Loader2,
  XCircle,
} from 'lucide-react';
import type {
  MilestoneItem,
  ReasoningItem,
  ToolActivityItem,
} from './eventReducer';
import { cn } from '../../lib/utils';
import { MarkdownMessage } from './MarkdownMessage';
import { useDeckStore } from '../../stores/deckStore';
import { useProjectStore } from '../../stores/projectStore';

function safeReasoningMarkdown(text: string): string {
  return text.replace(/```[\s\S]*?```/g, '').trim();
}

export const ReasoningRow: React.FC<{ item: ReasoningItem }> = ({ item }) => {
  const [expanded, setExpanded] = useState(false);
  const collapsible = item.text.length > 220 || item.text.split('\n').length > 4;
  return (
    <div className="flex items-start gap-2 py-1 text-[13px] leading-[1.55] text-text-600">
      <BrainCircuit className="mt-0.5 h-4 w-4 shrink-0 text-text-400" strokeWidth={1.75} />
      <div className="min-w-0 flex-1">
        <div className={cn(!expanded && collapsible && 'line-clamp-4')}>
          <MarkdownMessage content={safeReasoningMarkdown(item.text)} />
        </div>
        {collapsible && (
          <button
            type="button"
            onClick={() => setExpanded((value) => !value)}
            className="mt-1 text-xs text-accent hover:underline"
          >
            {expanded ? '收起思路' : '展开思路'}
          </button>
        )}
      </div>
    </div>
  );
};

export const MilestoneRow: React.FC<{ item: MilestoneItem }> = ({ item }) => (
  <div className="mt-3 flex items-start gap-2 border-t border-border pt-3 text-[13px] font-medium leading-5 text-text-900 motion-safe:animate-[timeline-enter_120ms_ease-out]">
    <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-success" strokeWidth={1.75} />
    <span>{item.text}</span>
  </div>
);

export const ToolActivityRow: React.FC<{ item: ToolActivityItem }> = ({ item }) => {
  const [expanded, setExpanded] = useState(item.status === 'failed' || Boolean(item.preview?.warnings.length));
  const activeProjectId = useProjectStore((state) => state.activeProjectId);
  const slides = useProjectStore((state) => activeProjectId ? state.slidesByProjectId[activeProjectId] ?? [] : []);
  const setCurrentPage = useDeckStore((state) => state.setCurrentPage);
  const hasDetails = Boolean(item.detail || item.error || item.preview);
  const icon = item.status === 'running'
    ? <Loader2 className="h-4 w-4 animate-spin text-accent motion-reduce:animate-none" strokeWidth={1.75} />
    : item.status === 'failed'
      ? <XCircle className="h-4 w-4 text-danger" strokeWidth={1.75} />
      : <CheckCircle2 className="h-4 w-4 text-text-400" strokeWidth={1.75} />;
  const focusPreview = () => {
    if (!item.preview) return;
    const index = slides.findIndex((slide) => slide.id === item.preview?.slide_id);
    if (index >= 0) setCurrentPage(index);
  };

  return (
    <div className={cn(
      'rounded-lg transition-colors duration-150',
      item.status === 'running' && 'bg-accent-soft/60',
    )}>
      <button
        type="button"
        disabled={!hasDetails}
        onClick={() => setExpanded((value) => !value)}
        className="flex min-h-8 w-full items-center gap-2 px-1.5 py-1 text-left disabled:cursor-default"
      >
        {icon}
        <span className={cn(
          'min-w-0 flex-1 truncate text-[13px]',
          item.status === 'failed' ? 'text-danger' : 'text-text-900',
        )}>
          {item.label}
        </span>
        {hasDetails && (expanded
          ? <ChevronDown className="h-3.5 w-3.5 text-text-400" />
          : <ChevronRight className="h-3.5 w-3.5 text-text-400" />)}
      </button>
      {expanded && hasDetails && (
        <div className="ml-6 space-y-2 px-1.5 pb-2 text-xs leading-5 text-text-600">
          {(item.error?.message || item.detail) && <p>{item.error?.message ?? item.detail}</p>}
          {item.error?.retryable && <p>Agent 可以调整后继续尝试。</p>}
          {item.preview && (
            <div className="overflow-hidden rounded-lg border border-border bg-surface">
              <button
                type="button"
                onClick={focusPreview}
                disabled={!slides.some((slide) => slide.id === item.preview?.slide_id)}
                aria-label={`在工作区查看 ${item.preview.slide_id}`}
                className="block w-full disabled:cursor-default"
              >
                <img
                  src={item.preview.image_url}
                  alt={`${item.preview.slide_id} 渲染预览`}
                  className="aspect-video w-full object-cover"
                />
              </button>
              <div className="flex items-center justify-between gap-2 px-2 py-1.5">
                <span>{item.preview.slide_id}</span>
                <span>{item.preview.warnings.length > 0 ? `${item.preview.warnings.length} 项布局提示` : '布局正常'}</span>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
};

export const ToolGroupRow: React.FC<{ items: ToolActivityItem[] }> = ({ items }) => {
  const [expanded, setExpanded] = useState(false);
  return (
    <div>
      <button
        type="button"
        aria-expanded={expanded}
        onClick={() => setExpanded((value) => !value)}
        className="flex min-h-8 w-full items-center gap-2 px-1.5 py-1 text-left text-[13px] text-text-900"
      >
        <CheckCircle2 className="h-4 w-4 text-text-400" strokeWidth={1.75} />
        <span className="min-w-0 flex-1">已完成 {items.length} 项{items[0].label.replace(/^已/, '')}</span>
        <span className="text-xs text-text-400">{expanded ? '收起' : '展开'}</span>
      </button>
      {expanded && (
        <div className="ml-4 border-l border-border pl-2">
          {items.map((item) => <ToolActivityRow key={item.id} item={item} />)}
        </div>
      )}
    </div>
  );
};
