import React, { useEffect, useLayoutEffect, useRef, useState } from 'react';
import {
  AlertTriangle,
  BrainCircuit,
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  Eye,
  ExternalLink,
  Flag,
  Loader2,
  RotateCcw,
  Search,
  SquareTerminal,
  Sparkles,
} from 'lucide-react';
import type {
  MilestoneItem,
  ReasoningItem,
  RunLifecycleItem,
  ToolActivityItem,
} from './eventReducer';
import type { PublicTarget, Slide } from '../../api/types';
import { cn } from '../../lib/utils';
import { MarkdownMessage } from './MarkdownMessage';
import { presentUserText } from './runtimeLabels';
import { useDeckStore } from '../../stores/deckStore';
import { useProjectStore } from '../../stores/projectStore';
import { orderedSlides } from '../deck/selectors';
import { TimelineDisclosure } from './TimelineDisclosure';
import { targetFileLabel } from './targetFileLabel';

function safeReasoningMarkdown(text: string): string {
  return text.replace(/```[\s\S]*?```/g, '').trim();
}

function pageName(slideId: string, slides: Slide[]): string {
  const index = slides.findIndex((slide) => slide.id === slideId);
  return index >= 0 ? `第 ${index + 1} 页` : `页面 ${slideId}`;
}

export function presentActivityText(text: string, target: PublicTarget | undefined, slides: Slide[]): string {
  const presented = presentUserText(text);
  if (!target || target.type !== 'slide' || !target.slide_id) return presented;
  const index = slides.findIndex((slide) => slide.id === target.slide_id);
  // 页码随 outline 顺序实时换算；新页尚未进入有序列表时，使用后端给出的安全展示名，避免泄露 slide_id。
  const replacement = index >= 0 ? `第 ${index + 1} 页` : target.display_name || '页面';
  const candidates = [`页面 ${target.slide_id}`, target.display_name].filter(
    (candidate): candidate is string => Boolean(candidate && candidate !== replacement),
  );
  for (const candidate of candidates) {
    if (presented.includes(candidate)) return presented.replace(candidate, replacement);
  }
  return presented;
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
  const toggle = () => setExpanded((value) => !value);
  // 整行可点击伸缩（与工具行一致）；仅当内容可切换时才附带交互，避免不可展开时误导。
  const interactive = showToggle
    ? {
        role: 'button' as const,
        tabIndex: 0,
        'aria-expanded': expanded,
        'aria-label': expanded ? '收起思路' : '展开思路',
        onClick: toggle,
        onKeyDown: (event: React.KeyboardEvent) => {
          if (event.key === 'Enter' || event.key === ' ') {
            event.preventDefault();
            toggle();
          }
        },
      }
    : {};

  return (
    <div
      {...interactive}
      className={cn(
        'grid grid-cols-[16px_minmax(0,1fr)_16px] items-start gap-2 rounded-lg px-1.5 py-1 text-[13px] leading-5 text-text-600',
        showToggle && 'cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent',
      )}
    >
      <span className="flex h-5 w-4 items-center justify-center" aria-hidden="true">
        <BrainCircuit className="h-4 w-4 text-accent" strokeWidth={1.75} />
      </span>
      <div ref={textRef} className={cn('min-w-0 flex-1', !expanded && 'line-clamp-1')}>
        <MarkdownMessage
          content={safeReasoningMarkdown(item.text)}
          className="text-text-600 prose-headings:my-0 prose-p:my-0 prose-p:leading-5 prose-ul:my-0 prose-ol:my-0 prose-li:my-0 prose-strong:text-text-600"
        />
      </div>
      {showToggle && (
        <span className="flex h-5 w-4 items-center justify-center text-text-400" aria-hidden="true">
          {expanded
            ? <ChevronDown className="h-3.5 w-3.5" strokeWidth={1.75} />
            : <ChevronRight className="h-3.5 w-3.5" strokeWidth={1.75} />}
        </span>
      )}
      {!showToggle && (
        <span aria-hidden="true" />
      )}
    </div>
  );
};

export const MilestoneRow: React.FC<{ item: MilestoneItem }> = ({ item }) => {
  const [expanded, setExpanded] = useState(false);
  const [overflowing, setOverflowing] = useState(false);
  const textRef = useRef<HTMLSpanElement>(null);

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
  const toggle = () => setExpanded((value) => !value);

  const interactive = showToggle
    ? {
        role: 'button' as const,
        tabIndex: 0,
        'aria-expanded': expanded,
        'aria-label': expanded ? '收起计划' : '展开计划',
        onClick: toggle,
        onKeyDown: (event: React.KeyboardEvent) => {
          if (event.key === 'Enter' || event.key === ' ') {
            event.preventDefault();
            toggle();
          }
        },
      }
    : {};

  return (
    <div
      {...interactive}
      className={cn(
        'flex items-start gap-2 rounded-lg px-1.5 py-1 text-[13px] leading-5 text-text-900',
        showToggle && 'cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent',
      )}
    >
      <Flag className="mt-0.5 h-4 w-4 shrink-0 text-[#7C3AED]" strokeWidth={1.75} />
      <span ref={textRef} className={cn('min-w-0 flex-1', !expanded && 'line-clamp-1')}>{item.text}</span>
      {showToggle && (
        <span className="mt-0.5 shrink-0 text-text-400" aria-hidden="true">
          {expanded
            ? <ChevronDown className="h-3.5 w-3.5" strokeWidth={1.75} />
            : <ChevronRight className="h-3.5 w-3.5" strokeWidth={1.75} />}
        </span>
      )}
    </div>
  );
};

export const RunLifecycleRow: React.FC<{ item: RunLifecycleItem }> = ({ item }) => (
  <div className="flex min-h-8 items-center gap-2 px-1.5 py-1 text-[13px] text-text-600">
    <RotateCcw className="h-4 w-4 shrink-0 text-accent" strokeWidth={1.75} />
    <span className="min-w-0 flex-1 truncate font-medium">{item.text}</span>
  </div>
);

// 图标字形按工具区分（读取=eye，创建=sparkles，搜索=search），颜色由状态决定：
// 成功=success 绿、失败=danger 红。兜底工具（render）成功用勾、失败用三角。
function toolStatusIcon(tool: string, failed: boolean) {
  const className = cn('h-4 w-4', failed ? 'text-danger' : 'text-success');
  if (tool === 'read_ppt') return <Eye className={className} strokeWidth={1.75} />;
  if (tool === 'mutate_ppt') return <Sparkles className={className} strokeWidth={1.75} />;
  if (tool === 'search_reference' || tool.startsWith('search') || tool.includes('reference')) {
    return <Search className={className} strokeWidth={1.75} />;
  }
  return failed
    ? <AlertTriangle className={className} strokeWidth={1.75} />
    : <CheckCircle2 className={className} strokeWidth={1.75} />;
}

export const ToolActivityRow: React.FC<{ item: ToolActivityItem }> = ({ item }) => {
  const [expanded, setExpanded] = useState(Boolean(item.preview?.warnings.length));
  const [runningVisible, setRunningVisible] = useState(
    item.tool !== 'run_command' || item.status !== 'running' || Date.now() - item.timestamp >= 300,
  );
  const activeProjectId = useProjectStore((state) => state.activeProjectId);
  const snapshot = useProjectStore((state) => activeProjectId ? state.contentByProjectId[activeProjectId] : undefined);
  const slides = orderedSlides(snapshot);
  const setCurrentSlideId = useDeckStore((state) => state.setCurrentSlideId);
  const hasDetails = Boolean(item.detail || item.error || item.preview || item.command);
  const detailText = item.error?.message ?? item.detail;
  const commandOutput = [
    item.command?.stdout_preview,
    item.command?.stderr_preview,
    item.command?.output_truncated ? '输出已裁剪。' : undefined,
    !item.command?.stdout_preview && !item.command?.stderr_preview ? detailText : undefined,
  ].filter(Boolean).join('\n');
  const commandColor = item.status === 'running'
    ? 'text-accent'
    : item.status === 'completed'
      ? 'text-success'
      : item.status === 'blocked'
        ? 'text-warning'
        : 'text-danger';
  const icon = item.tool === 'run_command'
    ? <SquareTerminal className={`h-4 w-4 ${commandColor}`} strokeWidth={1.75} />
    : item.status === 'running'
      ? <Loader2 className="h-4 w-4 animate-spin text-warning motion-reduce:animate-none" strokeWidth={1.75} />
      : toolStatusIcon(item.tool, item.status === 'failed' || item.status === 'blocked');

  useEffect(() => {
    if (item.tool !== 'run_command' || item.status !== 'running') {
      setRunningVisible(true);
      return undefined;
    }
    const remaining = Math.max(0, 300 - (Date.now() - item.timestamp));
    const timer = window.setTimeout(() => setRunningVisible(true), remaining);
    return () => window.clearTimeout(timer);
  }, [item.status, item.timestamp, item.tool]);
  const focusPreview = () => {
    if (!item.preview) return;
    const slideId = item.preview.slide_id;
    if (slides.some((slide) => slide.id === slideId)) setCurrentSlideId(slideId);
  };

  if (!runningVisible) return null;

  return (
    <div className="overflow-hidden rounded-lg">
      <button
        type="button"
        disabled={!hasDetails}
        onClick={() => setExpanded((value) => !value)}
        className="grid min-h-8 w-full grid-cols-[16px_minmax(0,1fr)_16px] items-center gap-2 bg-transparent px-1.5 py-1 text-left disabled:cursor-default"
      >
        {icon}
        <span className="min-w-0 truncate text-[13px] font-normal text-text-900">
          {presentActivityText(item.label, item.target, slides)}
        </span>
        {hasDetails && (expanded
          ? <ChevronDown className="h-3.5 w-3.5 text-text-400" />
          : <ChevronRight className="h-3.5 w-3.5 text-text-400" />)}
      </button>
      <TimelineDisclosure open={expanded && hasDetails}>
        {expanded && hasDetails && <div className="pb-1.5 pl-[30px] pr-2 pt-px text-xs leading-5 text-text-600">
          {item.command ? (
            <div className="rounded-md bg-[#EDF0F3] px-2.5 py-[9px] font-mono text-[11px] leading-[1.6] text-[#526071]">
              <code className="block whitespace-pre-wrap break-words font-semibold text-[#263241]">
                {item.command.text}
              </code>
              {commandOutput && (
                <pre className={cn(
                  'mt-[7px] whitespace-pre-wrap break-words border-t border-[#D7DCE3] pt-[7px] font-mono text-[11px] font-normal text-[#758191]',
                  item.status === 'failed' && 'text-[#A34851]',
                )}>
                  {commandOutput}
                </pre>
              )}
            </div>
          ) : detailText && (
            item.target?.open_url ? (
              <a
                href={item.target.open_url}
                className="inline-flex max-w-full items-center gap-1 text-text-600 underline decoration-border underline-offset-2 hover:text-text-900"
                title={item.target.local_path ?? detailText}
              >
                <span className="truncate">
                  {targetFileLabel(item.target) ?? presentActivityText(detailText, item.target, slides)}
                </span>
                <ExternalLink className="h-3.5 w-3.5 shrink-0" strokeWidth={1.75} />
              </a>
            ) : <p>{presentActivityText(detailText, item.target, slides)}</p>
          )}
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
        </div>}
      </TimelineDisclosure>
    </div>
  );
};

const groupVerbByTool: Record<string, string> = {
  read_ppt: '已读取',
};

function groupedVerb(items: ToolActivityItem[]): string {
  if (items[0].tool !== 'mutate_ppt') return groupVerbByTool[items[0].tool] ?? '已完成';
  const verbs = items.map((item) => item.label.startsWith('已创建') ? '已创建' : item.label.startsWith('已更新') ? '已更新' : '已完成');
  return verbs.every((verb) => verb === verbs[0]) ? verbs[0] : '已完成';
}

function groupedObjectLabel(items: ToolActivityItem[]): string {
  const kinds = items.map((item) => {
    const target = item.target;
    if (target?.type === 'slide' && target.part === 'spec') return '页面设计稿';
    if (target?.type === 'slide' && target.part === 'html') return '幻灯片';
    if (target?.type === 'deck' && target.part === 'manifest') return '演示内容';
    if (target?.type === 'deck' && target.part === 'outline') return '目录结构';
    if (target?.type === 'deck' && target.part === 'design') return '视觉设计';
    return '';
  });
  const first = kinds[0];
  if (!first || kinds.some((kind) => kind !== first)) return `${items.length} 项`;
  const unit = first === '幻灯片' ? '张' : first === '目录结构' ? '份' : first === '视觉设计' ? '套' : '个';
  return `${items.length} ${unit}${first}`;
}

export const ToolGroupRow: React.FC<{ items: ToolActivityItem[] }> = ({ items }) => {
  const [expanded, setExpanded] = useState(false);
  const verb = groupedVerb(items);
  return (
    <div>
      <button
        type="button"
        aria-expanded={expanded}
        onClick={() => setExpanded((value) => !value)}
        className="flex min-h-8 w-full items-center gap-2 px-1.5 py-1 text-left text-[13px] text-text-900"
      >
        {items[0].tool === 'run_command'
          ? <SquareTerminal className="h-4 w-4 text-success" strokeWidth={1.75} />
          : toolStatusIcon(items[0].tool, false)}
        <span className="min-w-0 flex-1">
          {items[0].tool === 'run_command'
            ? `已执行 ${items.length} 条命令`
            : `${verb} ${groupedObjectLabel(items)}`}
        </span>
        {expanded
          ? <ChevronDown className="h-3.5 w-3.5 shrink-0 text-text-400" strokeWidth={1.75} />
          : <ChevronRight className="h-3.5 w-3.5 shrink-0 text-text-400" strokeWidth={1.75} />}
      </button>
      <TimelineDisclosure open={expanded}>
        {expanded && <div className="timeline-disclosure-rows">
          {items.map((item) => <ToolActivityRow key={item.id} item={item} />)}
        </div>}
      </TimelineDisclosure>
    </div>
  );
};
