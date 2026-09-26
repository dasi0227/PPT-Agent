import { isResourceEditTool } from '../../api/resourceTools';
import { FileOpenButton } from '../../components/ui/FileOpenButton';
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
  Monitor,
  RotateCcw,
  Search,
  SquareTerminal,
  Pencil,
  Component as ComponentIcon,
  BookOpenText,
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
import { useDeckStore } from '../../stores/deckStore';
import { useProjectStore } from '../../stores/projectStore';
import { orderedSlides } from '../deck/selectors';
import { TimelineDisclosure } from './TimelineDisclosure';
import { LongContent } from './LongContent';
import { isAuthoringDataTarget, targetFileLabel } from './targetFileLabel';
import { partLabel } from '../viewer/semanticLabels';

function safeReasoningMarkdown(text: string): string {
  return text.replace(/```[\s\S]*?```/g, '').trim();
}

function pageName(slideId: string, slides: Slide[]): string {
  const index = slides.findIndex((slide) => slide.id === slideId);
  return index >= 0 ? `第 ${index + 1} 页` : '已删除页面';
}

export function presentActivityText(text: string, target: PublicTarget | undefined, slides: Slide[]): string {
  if (target?.type === 'deck') {
    if (target.part === 'manifest') return text.replace(/演示内容/g, partLabel('manifest'));
    if (target.part === 'design') return text.replace(/视觉设计/g, partLabel('design'));
  }
  if (!target || target.type !== 'slide' || !target.slide_id) return text;
  if (target.part === 'spec') text = text.replace(/设计稿/g, partLabel('spec'));
  const index = slides.findIndex((slide) => slide.id === target.slide_id);
  // 当前快照确定页面顺序；已不在目录中的目标使用明确回退名称。
  const replacement = index >= 0 ? `第 ${index + 1} 页` : '已删除页面';
  const candidates = [`页面 ${target.slide_id}`, target.display_name].filter(
    (candidate): candidate is string => Boolean(candidate && candidate !== replacement),
  );
  for (const candidate of candidates) {
    if (text.includes(candidate)) return text.replace(candidate, replacement);
  }
  return text;
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
        'aria-expanded': expanded,
        'aria-label': expanded ? '收起思路' : '展开思路',
        onClick: toggle,
      }
    : {};

  return (
    <div
      {...interactive}
      className={cn(
        'grid grid-cols-[16px_minmax(0,1fr)_16px] items-start gap-2 rounded-lg px-1.5 py-1 text-[13px] leading-5 text-text-600',
        showToggle && 'cursor-pointer focus-visible:outline-none',
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

  const completedMatch = item.text.match(/^已完成「(.+)」$/);
  const completedTitles = completedMatch?.[1].split('」、「');

  const interactive = showToggle
    ? {
        role: 'button' as const,
        'aria-expanded': expanded,
        'aria-label': expanded ? '收起计划' : '展开计划',
        onClick: toggle,
      }
    : {};

  return (
    <div
      {...interactive}
      className={cn(
        'flex items-start gap-2 rounded-lg px-1.5 py-1 text-[13px] leading-5',
        showToggle && 'cursor-pointer focus-visible:outline-none',
      )}
    >
      <Flag className="mt-0.5 h-4 w-4 shrink-0 text-success" strokeWidth={1.75} />
      <span ref={textRef} className={cn('min-w-0 flex-1 text-text-900', !expanded && 'line-clamp-1')}>
        {completedTitles ? (
          <>已完成「{completedTitles.map((title, index) => (
            <React.Fragment key={`${title}:${index}`}>
              {index > 0 && '」、「'}
              <strong className="font-semibold">{title}</strong>
            </React.Fragment>
          ))}」</>
        ) : <strong className="font-semibold">{item.text}</strong>}
      </span>
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
  <div className="flex min-h-8 items-center gap-2 px-1.5 py-1 text-[13px] text-text-900">
    <RotateCcw className="h-4 w-4 shrink-0 text-accent" strokeWidth={1.75} />
    <span className="min-w-0 flex-1 truncate">{item.text}</span>
  </div>
);

// 图标字形按工具区分（读取=eye，编辑=pencil，渲染=monitor，搜索=search），颜色由状态决定：
// 成功=success 绿、失败=danger 红；未知工具才使用通用状态图标兜底。
function toolStatusIcon(tool: string, failed: boolean) {
  const className = cn('h-4 w-4', failed ? 'text-danger' : 'text-success');
  if (tool === 'read_resource') return <Eye className={className} strokeWidth={1.75} />;
  if (isResourceEditTool(tool)) return <Pencil className={className} strokeWidth={1.75} />;
  if (tool === 'render_slide') return <Monitor className={className} strokeWidth={1.75} />;
  if (tool === 'load_component') return <ComponentIcon className={className} strokeWidth={1.75} />;
  if (tool === 'load_skill') return <BookOpenText className={className} strokeWidth={1.75} />;
  if (tool === 'search_reference' || tool.startsWith('search') || tool.includes('reference')) {
    return <Search className={className} strokeWidth={1.75} />;
  }
  return failed
    ? <AlertTriangle className={className} strokeWidth={1.75} />
    : <CheckCircle2 className={className} strokeWidth={1.75} />;
}

// 命令卡片：默认限制最大高度只展示部分，超出时在底部提供「展开全部」，展开后可「收起」。
function CommandCard({ command, commandOutput, status }: {
  command: NonNullable<ToolActivityItem['command']>;
  commandOutput: string;
  status: ToolActivityItem['status'];
}) {
  return (
    <LongContent
      className="overflow-hidden rounded-md bg-[#EDF0F3]"
      contentClassName="px-2.5 py-[9px] font-mono text-[11px] leading-[1.6] text-[#526071]"
      fadeClassName="from-[#EDF0F3]/0 via-[#EDF0F3]/90 to-[#EDF0F3]"
      buttonClassName="h-6 text-[11px]"
      controlsClassName="mt-0 pb-[9px]"
    >
      <code className="block whitespace-pre-wrap break-words font-semibold text-[#263241]">
        {command.text}
      </code>
      {commandOutput && (
        <pre className={cn(
          'mt-[7px] whitespace-pre-wrap break-words border-t border-[#D7DCE3] pt-[7px] font-mono text-[11px] font-normal text-[#758191]',
          status === 'failed' && 'text-[#A34851]',
        )}>
          {commandOutput}
        </pre>
      )}
    </LongContent>
  );
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
  const detailText = item.error?.message ?? item.detail;
  const renderSlideId = item.target?.slide_id ?? item.preview?.slide_id;
  const renderPage = renderSlideId ? pageName(renderSlideId, slides) : '页面';
  const renderObject = renderPage === '已删除页面' ? '幻灯片（页面已删除）' : `${renderPage}幻灯片`;
  const name = item.tool === 'run_command' ? commandName(item.command?.text) : null;
  const label = item.tool === 'render_slide'
    ? item.status === 'completed' ? `已渲染${renderObject}`
      : item.status === 'running' ? `正在渲染${renderObject}`
        : `渲染${renderPage === '已删除页面' ? renderObject : renderPage}失败`
    : item.tool === 'run_command' && item.status === 'completed'
    ? name ? `已执行 ${name} 命令` : '已执行命令'
    : isResourceEditTool(item.tool) && item.status === 'completed'
    ? item.label.replace(/^已(?:创建|更新)/, '已编辑')
    : item.label;
  const renderPassed = item.tool === 'render_slide' && item.status === 'completed';
  const showDetailText = Boolean(detailText) && !renderPassed;
  const hasDetails = Boolean(showDetailText || item.preview || item.command || item.resources?.length);
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
          {presentActivityText(label, item.target, slides)}
        </span>
        {hasDetails && (expanded
          ? <ChevronDown className="h-3.5 w-3.5 text-text-400" />
          : <ChevronRight className="h-3.5 w-3.5 text-text-400" />)}
      </button>
      <TimelineDisclosure open={expanded && hasDetails}>
        {expanded && hasDetails && <div className={cn(
          'pb-1.5 text-xs leading-5 text-text-600',
          item.command || item.preview ? 'timeline-detail-card' : 'pl-[30px] pr-2 pt-px',
        )}>
          {item.command ? (
            <CommandCard command={item.command} commandOutput={commandOutput} status={item.status} />
          ) : showDetailText && detailText && (
            item.target?.open_url && !isAuthoringDataTarget(item.target) ? (
              <FileOpenButton
                url={item.target.open_url}
                className="inline-flex max-w-full items-center gap-1 text-text-600 underline decoration-border underline-offset-2 hover:text-text-900"
                label={targetFileLabel(item.target, item.target.slide_id ? pageName(item.target.slide_id, slides) : undefined) ?? detailText}
              >
                <span className="truncate">
                  {targetFileLabel(item.target, item.target.slide_id ? pageName(item.target.slide_id, slides) : undefined) ?? presentActivityText(detailText, item.target, slides)}
                </span>
                <ExternalLink className="h-3.5 w-3.5 shrink-0" strokeWidth={1.75} />
              </FileOpenButton>
            ) : <p>{isAuthoringDataTarget(item.target)
              ? targetFileLabel(item.target, item.target?.slide_id ? pageName(item.target.slide_id, slides) : undefined)
              : presentActivityText(detailText, item.target, slides)}</p>
          )}
          {item.error?.retryable && <p>Agent 可以调整后继续尝试。</p>}
          {item.resources && item.resources.length > 0 && (
            <ul className="space-y-0.5">
              {item.resources.map((resource) => (
                <li key={`${resource.kind}:${resource.id}`} className="flex min-h-5 items-center">
                  {resource.open_url ? (
                    <FileOpenButton url={resource.open_url} label={resource.name} className="inline-flex min-w-0 items-center gap-1 rounded px-1 hover:bg-accent-soft hover:text-text-900">
                      <span className="truncate">{resource.name}</span>
                      <ExternalLink className="h-3.5 w-3.5 shrink-0" strokeWidth={1.75} />
                    </FileOpenButton>
                  ) : <span className="truncate px-1">{resource.name}</span>}
                </li>
              ))}
            </ul>
          )}
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
  read_resource: '已读取',
  edit_manifest: '已编辑',
  edit_design: '已编辑',
  edit_spec: '已编辑',
  init_outline: '已编辑',
  arrange_outline: '已编辑',
  write_html: '已编辑',
  patch_html: '已编辑',
};

function targetObjectName(target: PublicTarget | undefined): string {
  if (target?.type === 'deck' && ['manifest', 'outline', 'design'].includes(target.part)) {
    return partLabel(target.part);
  }
  if (target?.type === 'slide' && ['spec', 'html'].includes(target.part)) {
    return partLabel(target.part);
  }
  return '';
}

// 从命令文本提取可执行程序名，供单条调用展示；完整命令保留在展开详情中。
function commandName(text?: string): string | null {
  const trimmed = text?.trim();
  if (!trimmed) return null;
  const match = trimmed.match(/^\S+/);
  return match ? match[0] : null;
}

interface GroupedObjectParts {
  prefix: string;
  noun: string | null;
}

function groupedObjectParts(items: ToolActivityItem[]): GroupedObjectParts {
  const kinds = items.map((item) => targetObjectName(item.target));
  const first = kinds[0];
  if (!first || kinds.some((kind) => kind !== first)) {
    return { prefix: `${items.length} 项`, noun: null };
  }
  const unit = first === '幻灯片' ? '张' : first === '目录结构' ? '份' : first === partLabel('design') ? '套' : '个';
  return { prefix: `${items.length} ${unit}`, noun: first };
}

function groupLabel(items: ToolActivityItem[], verb: string): string {
  if (items[0].tool === 'render_slide') {
    return `已渲染 ${items.length} 张幻灯片`;
  }
  if (items[0].tool === 'run_command') {
    return `已执行 ${items.length} 条命令`;
  }
  const { prefix, noun } = groupedObjectParts(items);
  return `${verb} ${prefix}${noun ?? ''}`;
}

export const ToolGroupRow: React.FC<{ items: ToolActivityItem[] }> = ({ items }) => {
  const [expanded, setExpanded] = useState(false);
  const verb = groupVerbByTool[items[0].tool] ?? '已完成';
  return (
    <div>
      <button
        type="button"
        aria-expanded={expanded}
        onClick={() => setExpanded((value) => !value)}
        className="flex min-h-8 w-full items-center gap-2 px-1.5 py-1 text-left text-[13px] font-normal text-text-900"
      >
        {items[0].tool === 'run_command'
          ? <SquareTerminal className="h-4 w-4 text-success" strokeWidth={1.75} />
          : toolStatusIcon(items[0].tool, false)}
        <span className="min-w-0 flex-1">
          {groupLabel(items, verb)}
        </span>
        {expanded
          ? <ChevronDown className="h-3.5 w-3.5 shrink-0 text-text-400" strokeWidth={1.75} />
          : <ChevronRight className="h-3.5 w-3.5 shrink-0 text-text-400" strokeWidth={1.75} />}
      </button>
      <TimelineDisclosure open={expanded}>
        {expanded && <div className="timeline-disclosure-rows pt-2">
          {items.map((item) => <ToolActivityRow key={item.id} item={item} />)}
        </div>}
      </TimelineDisclosure>
    </div>
  );
};
