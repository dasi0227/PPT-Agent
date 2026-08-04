import React, { useLayoutEffect, useRef, useState } from 'react';
import {
  BrainCircuit,
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  Eye,
  Loader2,
  Pencil,
  Sparkles,
  XCircle,
} from 'lucide-react';
import type {
  MilestoneItem,
  ReasoningItem,
  ToolActivityItem,
} from './eventReducer';
import type { PublicTarget, Slide } from '../../api/types';
import { cn } from '../../lib/utils';
import { MarkdownMessage } from './MarkdownMessage';
import { presentUserText } from './runtimeLabels';
import { useDeckStore } from '../../stores/deckStore';
import { useProjectStore } from '../../stores/projectStore';

function safeReasoningMarkdown(text: string): string {
  return text.replace(/```[\s\S]*?```/g, '').trim();
}

function pageName(slideId: string, slides: Slide[]): string {
  const index = slides.findIndex((slide) => slide.id === slideId);
  return index >= 0 ? `第 ${index + 1} 页` : `页面 ${slideId}`;
}

function presentActivityText(text: string, target: PublicTarget | undefined, slides: Slide[]): string {
  const presented = presentUserText(text);
  if (!target || target.type !== 'slide' || !target.slide_id) return presented;
  const index = slides.findIndex((slide) => slide.id === target.slide_id);
  // 页码随 outline 顺序实时换算；新页尚未进入有序列表时，抹去占位符只留产物裸名，绝不暴露 slide_id。
  const replacement = index >= 0 ? `第 ${index + 1} 页` : '';
  return presented.split(`页面 ${target.slide_id}`).join(replacement);
}

export const ReasoningRow: React.FC<{ item: ReasoningItem }> = ({ item }) => {
  const [expanded, setExpanded] = useState(false);
  const [overflowing, setOverflowing] = useState(false);
  const textRef = useRef<HTMLDivElement>(null);

  useLayoutEffect(() => {
    if (expanded) return;
    const el = textRef.current;
    if (!el) return;
    const measure = () => setOverflowing(el.scrollHeight - el.clientHeight > 1);
    measure();
    if (typeof ResizeObserver === 'undefined') return;
    const observer = new ResizeObserver(measure);
    observer.observe(el);
    return () => observer.disconnect();
  }, [expanded, item.text]);

  const showToggle = overflowing || expanded;
  return (
    <div className="flex items-start gap-2 px-1.5 py-1 text-[13px] leading-[1.55] text-text-600">
      <BrainCircuit className="mt-0.5 h-4 w-4 shrink-0 text-text-400" strokeWidth={1.75} />
      <div ref={textRef} className={cn('min-w-0 flex-1', !expanded && 'line-clamp-1')}>
        <MarkdownMessage content={safeReasoningMarkdown(item.text)} />
      </div>
      {showToggle && (
        <button
          type="button"
          onClick={() => setExpanded((value) => !value)}
          aria-expanded={expanded}
          aria-label={expanded ? '收起思路' : '展开思路'}
          className="mt-0.5 shrink-0 text-text-400 transition-colors hover:text-text-600"
        >
          {expanded
            ? <ChevronDown className="h-3.5 w-3.5" strokeWidth={1.75} />
            : <ChevronRight className="h-3.5 w-3.5" strokeWidth={1.75} />}
        </button>
      )}
    </div>
  );
};

export const MilestoneRow: React.FC<{ item: MilestoneItem }> = ({ item }) => (
  <div className="mt-3 flex items-start gap-2 border-t border-border px-1.5 pt-3 text-[13px] font-medium leading-5 text-text-900 motion-safe:animate-[timeline-enter_120ms_ease-out]">
    <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-success" strokeWidth={1.75} />
    <span>{item.text}</span>
  </div>
);

// 成功态按工具区分字形（颜色统一为 success 绿）：读取=eye，创建=sparkles，编辑=pencil。
function toolSuccessIcon(tool: string) {
  const className = 'h-4 w-4 text-success';
  if (tool === 'read_ppt') return <Eye className={className} strokeWidth={1.75} />;
  if (tool === 'edit_ppt') return <Pencil className={className} strokeWidth={1.75} />;
  if (tool === 'write_ppt') return <Sparkles className={className} strokeWidth={1.75} />;
  return <CheckCircle2 className={className} strokeWidth={1.75} />;
}

export const ToolActivityRow: React.FC<{ item: ToolActivityItem }> = ({ item }) => {
  const [expanded, setExpanded] = useState(item.status === 'failed' || Boolean(item.preview?.warnings.length));
  const activeProjectId = useProjectStore((state) => state.activeProjectId);
  const slides = useProjectStore((state) => activeProjectId ? state.slidesByProjectId[activeProjectId] ?? [] : []);
  const setCurrentPage = useDeckStore((state) => state.setCurrentPage);
  const hasDetails = Boolean(item.detail || item.error || item.preview);
  const detailText = item.error?.message ?? item.detail;
  const icon = item.status === 'running'
    ? <Loader2 className="h-4 w-4 animate-spin text-warning motion-reduce:animate-none" strokeWidth={1.75} />
    : item.status === 'failed'
      ? <XCircle className="h-4 w-4 text-danger" strokeWidth={1.75} />
      : toolSuccessIcon(item.tool);
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
          {presentActivityText(item.label, item.target, slides)}
        </span>
        {hasDetails && (expanded
          ? <ChevronDown className="h-3.5 w-3.5 text-text-400" />
          : <ChevronRight className="h-3.5 w-3.5 text-text-400" />)}
      </button>
      {expanded && hasDetails && (
        <div className="ml-6 space-y-2 px-1.5 pb-2 text-xs leading-5 text-text-600">
          {detailText && <p>{presentActivityText(detailText, item.target, slides)}</p>}
          {item.error?.retryable && <p>Agent 可以调整后继续尝试。</p>}
          {item.preview && (
            <div className="overflow-hidden rounded-lg border border-border bg-surface">
              <button
                type="button"
                onClick={focusPreview}
                disabled={!slides.some((slide) => slide.id === item.preview?.slide_id)}
                aria-label={`在工作区查看 ${pageName(item.preview.slide_id, slides)}`}
                className="block w-full disabled:cursor-default"
              >
                <img
                  src={item.preview.image_url}
                  alt={`${pageName(item.preview.slide_id, slides)}渲染预览`}
                  className="aspect-video w-full object-cover"
                />
              </button>
              <div className="flex items-center justify-between gap-2 px-2 py-1.5">
                <span>{pageName(item.preview.slide_id, slides)}</span>
                <span>{item.preview.warnings.length > 0 ? `${item.preview.warnings.length} 项布局提示` : '布局正常'}</span>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
};

// 组按工具汇聚（同组同 tool），产物可能混合，故用中性量词「项」。
const groupVerbByTool: Record<string, string> = {
  read_ppt: '已读取',
  write_ppt: '已创建',
  edit_ppt: '已更新',
};

export const ToolGroupRow: React.FC<{ items: ToolActivityItem[] }> = ({ items }) => {
  const [expanded, setExpanded] = useState(false);
  const verb = groupVerbByTool[items[0].tool] ?? '已完成';
  return (
    <div>
      <button
        type="button"
        aria-expanded={expanded}
        onClick={() => setExpanded((value) => !value)}
        className="flex min-h-8 w-full items-center gap-2 px-1.5 py-1 text-left text-[13px] text-text-900"
      >
        {toolSuccessIcon(items[0].tool)}
        <span className="min-w-0 flex-1">
          {verb} {items.length} 项
        </span>
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
