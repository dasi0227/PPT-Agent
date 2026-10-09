import {
  Activity,
  Ellipsis,
  FileText,
  Gauge,
  History,
  Loader2,
  ShieldCheck,
} from 'lucide-react';
import { useEffect, useId, useMemo, useRef, useState, type RefObject } from 'react';
import type { ContextBucketKey, ContextWindowSnapshot } from '../../api/types';
import { IconButton } from '../../components/ui/primitives';
import { AnchoredPopover, AnchoredPopoverContent, AnchoredPopoverTitle, AnchoredPopoverTrigger } from '../../components/ui/anchored-popover';
import { useBriefingStore } from '../../stores/briefingStore';
import { useComposerStore } from '../../stores/composerStore';
import { useContextWindowStore } from '../../stores/contextWindowStore';
import { useGitCommitStore } from '../../stores/gitCommitStore';
import { useProjectStore } from '../../stores/projectStore';
import { useThreadStore } from '../../stores/threadStore';
import { runManualCompaction } from './manualCompaction';
import { useActiveSession } from './useActiveSession';
import { ToolbarProgressBadge } from './ToolbarProgressBadge';

const BUCKETS: Array<{
  key: ContextBucketKey;
  label: string;
  color: string;
  icon: typeof FileText;
}> = [
  { key: 'system_prompt', label: '系统提示词', color: 'rgb(var(--ui-chart-system))', icon: ShieldCheck },
  { key: 'runtime', label: '运行时', color: 'rgb(var(--ui-chart-runtime))', icon: Activity },
  { key: 'chat_history', label: '历史记录', color: 'rgb(var(--ui-chart-history))', icon: History },
  { key: 'read_file', label: '读取', color: 'rgb(var(--ui-chart-files))', icon: FileText },
  { key: 'other', label: '其它', color: 'rgb(var(--ui-chart-other))', icon: Ellipsis },
];

const DETAIL_DESCRIPTIONS: Record<string, string> = {
  'system prompts': '定义 Agent 行为、模式与任务约束',
  'tool definitions': '本轮可用工具及参数结构',
  'runtime context': '当前状态、计划、技能、组件与检索信息',
  'runtime messages': '运行时注入的控制指令与历史摘要',
  'user messages': '已提交的用户指令与补充',
  'assistant messages': 'Agent 已生成的自然语言回复',
  'tools execution': '工具及命令的调用与返回结果',
  read_resource: '通过 read_resource 读取的页面与项目内容',
  read_image: '读取或上传并送入模型的图片',
  other: '协议包装及尚未归类的剩余内容',
};

const EMPTY_BUCKETS: ContextWindowSnapshot['buckets'] = {
  system_prompt: 0,
  runtime: 0,
  chat_history: 0,
  read_file: 0,
  other: 0,
};

const EMPTY_DETAILS: ContextWindowSnapshot['details'] = {
  system_prompt: [{ name: 'system prompts', tokens: 0 }, { name: 'tool definitions', tokens: 0 }],
  runtime: [
    { name: 'runtime context', tokens: 0 },
    { name: 'runtime messages', tokens: 0 },
  ],
  chat_history: [
    { name: 'user messages', tokens: 0 },
    { name: 'assistant messages', tokens: 0 },
    { name: 'tools execution', tokens: 0 },
  ],
  read_file: [
    { name: 'read_resource', tokens: 0 },
    { name: 'read_image', tokens: 0 },
  ],
  other: [{ name: 'other', tokens: 0 }],
};

const EMPTY_SNAPSHOT: ContextWindowSnapshot = {
  total: 0,
  max: 0,
  ratio: 0,
  compactable_tokens: 0,
  compact_threshold_tokens: 0,
  status: 'idle',
  buckets: EMPTY_BUCKETS,
  details: EMPTY_DETAILS,
};

function formatParentTokens(tokens: number): string {
  return `${(tokens / 1000).toFixed(1)} k`;
}

function formatDetailTokens(tokens: number): string {
  return `${(tokens / 1000).toFixed(2)} k`;
}

function formatDetailName(name: string): string {
  return name.replace(/_/g, ' ');
}

function detailDescription(name: string): string {
  if (DETAIL_DESCRIPTIONS[name]) return DETAIL_DESCRIPTIONS[name];
  return '尚未归类的上下文内容';
}

export function ContextWindowPanel({ anchorRef, open: controlledOpen, onOpenChange }: {
  anchorRef?: RefObject<HTMLElement>;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}) {
  const [localOpen, setLocalOpen] = useState(false);
  const open = controlledOpen ?? localOpen;
  const panelId = useId();
  const detailPanelId = `${panelId}-details`;
  const triggerRef = useRef<HTMLButtonElement>(null);
  const activeProjectId = useProjectStore((state) => state.activeProjectId);
  const threadId = useThreadStore((state) => (
    activeProjectId ? state.activeThreadIdByProjectId[activeProjectId] : null
  ));
  const model = useComposerStore((state) => state.modelProfileName);
  const polishing = useComposerStore((state) => state.polishing);
  const { status: runStatus } = useActiveSession();
  const session = useContextWindowStore((state) => (
    threadId ? state.sessions[threadId] : undefined
  ));
  const load = useContextWindowStore((state) => state.load);
  const commitSession = useGitCommitStore((state) => (
    activeProjectId ? state.sessions[activeProjectId] : undefined
  ));
  const briefingActive = useBriefingStore((state) => (
    activeProjectId ? state.sessions[activeProjectId]?.status === 'generating' : false
  ));
  const [activeBucket, setActiveBucket] = useState<ContextBucketKey | null>(null);
  const [hoveredBucket, setHoveredBucket] = useState<ContextBucketKey | null>(null);
  const [focusedBucket, setFocusedBucket] = useState<ContextBucketKey | null>(null);
  const bucketRefs = useRef<Array<HTMLButtonElement | null>>([]);

  const clearSelection = () => {
    setActiveBucket(null);
    setHoveredBucket(null);
    setFocusedBucket(null);
  };

  const changeOpen = (nextOpen: boolean) => {
    clearSelection();
    if (controlledOpen === undefined) setLocalOpen(nextOpen);
    onOpenChange?.(nextOpen);
  };

  useEffect(() => {
    if (threadId && model) void load(threadId, model);
  }, [load, model, open, threadId]);

  useEffect(() => {
    setLocalOpen(false);
    onOpenChange?.(false);
    setActiveBucket(null);
    setHoveredBucket(null);
    setFocusedBucket(null);
  }, [activeProjectId, threadId, onOpenChange]);

  const snapshot = session?.snapshot ?? EMPTY_SNAPSHOT;
  const runActive = ['creating', 'running', 'waiting', 'paused', 'recovering', 'canceling'].includes(runStatus);
  const commitActive = commitSession?.status === 'creating' || commitSession?.status === 'running';
  const compacting = session?.compacting || snapshot.status === 'compacting';
  const compactionCapacityUnavailable = !session?.snapshot || snapshot.compact_threshold_tokens <= 0;
  const belowCompactThreshold = !compactionCapacityUnavailable
    && snapshot.compactable_tokens < snapshot.compact_threshold_tokens;
  const warning = snapshot.ratio >= 0.8;
  const disabled = !threadId || runActive || commitActive || briefingActive || polishing || compacting
    || compactionCapacityUnavailable || belowCompactThreshold;
  let compactButtonTitle = '手动压缩上下文';
  if (!threadId) compactButtonTitle = '请先选择任务';
  else if (compacting) compactButtonTitle = '正在压缩上下文';
  else if (runActive || commitActive || briefingActive || polishing) {
    compactButtonTitle = '当前操作完成后可手动压缩';
  } else if (compactionCapacityUnavailable) compactButtonTitle = '正在读取可压缩上下文';
  else if (belowCompactThreshold) {
    compactButtonTitle = `可压缩历史达到 ${formatParentTokens(snapshot.compact_threshold_tokens)} 后可用，当前 ${snapshot.compactable_tokens} Token`;
  }
  const percent = Math.round(snapshot.ratio * 100);
  const details = activeBucket ? snapshot.details[activeBucket] : [];
  const activeBucketMeta = BUCKETS.find((bucket) => bucket.key === activeBucket);
  const DetailIcon = activeBucketMeta?.icon ?? FileText;

  const segments = useMemo(() => {
    let offset = 0;
    return BUCKETS.map((bucket) => {
      const width = snapshot.max > 0 ? Math.max(0, snapshot.buckets[bucket.key] / snapshot.max * 100) : 0;
      const segment = { ...bucket, width, center: offset + width / 2 };
      offset += width;
      return segment;
    });
  }, [snapshot]);
  const previewSegment = segments.find((segment) => segment.key === (hoveredBucket ?? focusedBucket));

  const runCompact = async () => {
    if (!threadId || disabled) return;
    await runManualCompaction(threadId);
  };

  return (
    <AnchoredPopover open={open} onOpenChange={changeOpen}>
      <span className="relative inline-flex">
        <AnchoredPopoverTrigger asChild>
          <IconButton
            ref={triggerRef}
            label={`上下文窗口，当前使用 ${percent}%`}
            expandableLabel="上下文"
            data-state={open ? 'open' : 'closed'}
            aria-haspopup="dialog"
            aria-expanded={open}
            aria-controls={open ? panelId : undefined}
          >
            <span className="relative inline-flex h-4 w-4 shrink-0">
              <Gauge className="h-4 w-4" strokeWidth={1.75} />
              <ToolbarProgressBadge className={compacting ? 'text-success' : warning ? 'text-warning' : 'text-text-600'}>{percent}%</ToolbarProgressBadge>
            </span>
          </IconButton>
        </AnchoredPopoverTrigger>
      </span>

      {open && (
        <AnchoredPopoverContent
          anchorRef={anchorRef ?? triggerRef}
          side="bottom"
          align="end"
          sideOffset={6}
          showArrow={false}
          viewportPadding={0}
          viewportMode="layout"
          id={panelId}
          role="dialog"
          aria-label="上下文窗口"
          onOpenAutoFocus={(event) => event.preventDefault()}
          onCloseAutoFocus={(event) => event.preventDefault()}
          onInteractOutside={(event) => {
            if (triggerRef.current?.contains(event.target as Node)) return;
            event.preventDefault();
            clearSelection();
          }}
          onPointerDownCapture={(event) => {
            if (!(event.target as Element).closest('[data-context-bucket]')) clearSelection();
          }}
          className={`w-[min(24rem,calc(100vw-1.5rem))] rounded-xl ${
            warning && !compacting ? 'border-warning/50' : 'border-border-strong'
          }`}
        >
          <header className="flex min-h-11 items-center gap-2 px-3 py-2.5">
            <AnchoredPopoverTitle className="text-xs font-semibold text-text-900">上下文窗口</AnchoredPopoverTitle>
            {session?.loading && (
              <Loader2
                aria-label="正在加载上下文窗口"
                className="h-3.5 w-3.5 animate-spin text-text-400 motion-reduce:animate-none"
              />
            )}
            <button
              type="button"
              onClick={() => void runCompact()}
              disabled={disabled}
              className="ml-auto inline-flex h-7 items-center rounded-md border border-border bg-surface px-2.5 text-[11px] font-semibold text-text-600 ui-interactive focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-40"
              title={compactButtonTitle}
            >
              {compacting ? '压缩中' : '压缩'}
            </button>
          </header>

          <div className="flex items-center gap-2.5 px-3 pb-3 pt-3">
            <div className="relative h-2 min-w-0 flex-1 rounded-full bg-panel-muted" role="group" aria-label={`上下文占用，${percent}%`}>
              <div className="absolute inset-0 flex items-center overflow-hidden rounded-[inherit]">
                {segments.filter((segment) => segment.width > 0).map((segment) => (
                  <button
                    key={segment.key}
                    type="button"
                    data-context-bucket={segment.key}
                    aria-label={`${segment.label}，${formatParentTokens(snapshot.buckets[segment.key])}`}
                    aria-pressed={activeBucket === segment.key}
                    tabIndex={-1}
                    onClick={() => setActiveBucket(segment.key)}
                    onMouseEnter={() => setHoveredBucket(segment.key)}
                    onMouseLeave={() => setHoveredBucket(null)}
                    onFocus={() => setFocusedBucket(segment.key)}
                    onBlur={() => setFocusedBucket(null)}
                    className="shrink-0 border-0 p-0 first:rounded-l-full last:rounded-r-full outline-none transition-[width,height,opacity] duration-150 ease-out motion-reduce:transition-none"
                    style={{
                      width: `${segment.width}%`,
                      height: activeBucket && activeBucket !== segment.key ? '50%' : '100%',
                      opacity: activeBucket && activeBucket !== segment.key ? 0.28 : 1,
                      backgroundColor: segment.color,
                    }}
                  />
                ))}
              </div>
              <span className="pointer-events-none absolute inset-y-0 left-[85%] w-px bg-danger/70" />
              {previewSegment && (
                <>
                  <span
                    className="pointer-events-none absolute bottom-3.5 flex w-36 items-center justify-center gap-1.5 whitespace-nowrap text-[10px] leading-[14px] text-text-600"
                    style={{ left: `clamp(0px, calc(${previewSegment.center}% - 72px), calc(100% - 144px))` }}
                  >
                    <span className="h-1.5 w-1.5 shrink-0 rounded-[2px]" style={{ backgroundColor: previewSegment.color }} />
                    <span>{previewSegment.label}</span>
                    <span className="font-mono text-[9px] tabular-nums">{formatParentTokens(snapshot.buckets[previewSegment.key])}</span>
                  </span>
                  <span
                    className="pointer-events-none absolute bottom-2.5 h-1 w-px"
                    style={{ left: `${previewSegment.center}%`, backgroundColor: previewSegment.color }}
                  />
                </>
              )}
            </div>
            <span className={`min-w-10 text-right font-mono text-xs font-semibold tabular-nums ${
              compacting ? 'text-success' : warning ? 'text-warning' : 'text-text-600'
            }`}>
              {percent}%
            </span>
          </div>

          <div
            role="tablist"
            aria-label="上下文分桶"
            className="grid grid-cols-[1.35fr_1fr_1.1fr_0.85fr_0.85fr] gap-1 border-t border-border px-2.5 py-1.5"
          >
            {BUCKETS.map((bucket, index) => (
              <button
                key={bucket.key}
                type="button"
                ref={(element) => { bucketRefs.current[index] = element; }}
                role="tab"
                data-context-bucket={bucket.key}
                aria-selected={activeBucket === bucket.key}
                aria-controls={activeBucket === bucket.key ? detailPanelId : undefined}
                tabIndex={activeBucket === bucket.key || (activeBucket === null && index === 0) ? 0 : -1}
                onClick={() => setActiveBucket(bucket.key)}
                onMouseEnter={() => setHoveredBucket(bucket.key)}
                onMouseLeave={() => setHoveredBucket(null)}
                onFocus={() => setFocusedBucket(bucket.key)}
                onBlur={() => setFocusedBucket(null)}
                onKeyDown={(event) => {
                  let nextIndex: number | undefined;
                  if (event.key === 'ArrowRight') nextIndex = (index + 1) % BUCKETS.length;
                  if (event.key === 'ArrowLeft') nextIndex = (index + BUCKETS.length - 1) % BUCKETS.length;
                  if (event.key === 'Home') nextIndex = 0;
                  if (event.key === 'End') nextIndex = BUCKETS.length - 1;
                  if (nextIndex !== undefined) {
                    event.preventDefault();
                    setActiveBucket(BUCKETS[nextIndex].key);
                    bucketRefs.current[nextIndex]?.focus();
                  }
                }}
                className={`flex min-w-0 flex-col items-center gap-0.5 rounded-md px-0.5 py-1 text-[10px] font-semibold focus-visible:outline-none focus-visible:underline focus-visible:underline-offset-2 ${
                  activeBucket === bucket.key
                    ? 'ui-selected'
                    : 'text-text-600 ui-interactive'
                }`}
              >
                <span className="inline-flex items-center gap-1 whitespace-nowrap leading-3">
                  <span className="h-[7px] w-[7px] shrink-0 rounded-[2px]" style={{ backgroundColor: bucket.color }} />
                  <span>{bucket.label}</span>
                </span>
                <span className="font-mono text-[9px] font-normal leading-3 text-text-600 tabular-nums">
                  {formatParentTokens(snapshot.buckets[bucket.key])}
                </span>
              </button>
            ))}
          </div>

          {activeBucketMeta && <div
            id={detailPanelId}
            role="tabpanel"
            aria-label={`${activeBucketMeta.label}明细`}
            tabIndex={0}
            className="max-h-52 overflow-y-auto border-t border-border"
          >
            {details.map((detail) => (
              <div
                key={detail.name}
                className="grid grid-cols-[1.5rem_minmax(0,1fr)_4.75rem] items-center gap-2 border-b border-border px-3 py-2.5 last:border-b-0"
              >
                <span
                  className="grid h-6 w-6 shrink-0 place-items-center rounded-md bg-panel"
                  style={{ color: activeBucketMeta.color }}
                >
                  <DetailIcon className="h-3.5 w-3.5" strokeWidth={1.75} />
                </span>
                <span className="min-w-0">
                  <span className="block truncate text-[11px] font-semibold leading-4 text-text-900">
                    {formatDetailName(detail.name)}
                  </span>
                  <span className="block truncate text-[10px] leading-4 text-text-400">
                    {detailDescription(detail.name)}
                  </span>
                </span>
                <span className="text-right font-mono text-[10px] leading-4 text-text-600 tabular-nums">
                  {formatDetailTokens(detail.tokens)}
                </span>
              </div>
            ))}
          </div>}
        </AnchoredPopoverContent>
      )}
    </AnchoredPopover>
  );
}
