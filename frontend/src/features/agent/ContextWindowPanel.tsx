import {
  Activity,
  Ellipsis,
  FileText,
  Gauge,
  History,
  Loader2,
  Minimize2,
  ShieldCheck,
  Terminal,
} from 'lucide-react';
import { useEffect, useId, useMemo, useRef, useState } from 'react';
import type { ContextBucketKey, ContextWindowSnapshot } from '../../api/types';
import { threadsApi } from '../../api/threads';
import { IconButton } from '../../components/ui/primitives';
import { useBriefingStore } from '../../stores/briefingStore';
import { useComposerStore } from '../../stores/composerStore';
import { useContextWindowStore } from '../../stores/contextWindowStore';
import { useGitCommitStore } from '../../stores/gitCommitStore';
import { useProjectStore } from '../../stores/projectStore';
import { useRunStore } from '../../stores/runStore';
import { useThreadStore } from '../../stores/threadStore';
import { hydrateRunFromHistory, type HistoryEntry } from './historyHydrator';
import { useActiveSession } from './useActiveSession';

const BUCKETS: Array<{
  key: ContextBucketKey;
  label: string;
  color: string;
  icon: typeof FileText;
}> = [
  { key: 'system_prompt', label: '系统提示词', color: '#7C3AED', icon: ShieldCheck },
  { key: 'runtime', label: '运行时', color: '#2563EB', icon: Activity },
  { key: 'chat_history', label: '对话历史', color: '#DB2777', icon: History },
  { key: 'read_file', label: '读文件', color: '#D97706', icon: FileText },
  { key: 'run_command', label: '跑命令', color: '#0D9488', icon: Terminal },
  { key: 'other', label: '其它', color: '#8793A2', icon: Ellipsis },
];

const DETAIL_DESCRIPTIONS: Record<string, string> = {
  'system prompts': '定义 Agent 行为、模式与任务约束',
  'tool definitions': '本轮可用工具及参数结构',
  'runtime state': '当前模式、阶段、计划与执行进度',
  'runtime resources': '可用资源、引用与已加载能力',
  'runtime messages': '运行时自动注入的控制指令',
  'user messages': '已提交的用户指令与补充',
  'assistant messages': 'Agent 已生成的自然语言回复',
  'other tools': '其余工具的调用与返回结果',
  'context summary': '压缩历史生成的结构化摘要',
  read_ppt: '通过 read_ppt 读取的页面与项目内容',
  read_image: '读取或上传并送入模型的图片',
  read_project: '每轮自动注入的项目上下文',
  run_command: '尚未执行终端命令',
  'other command': '其余命令的调用与返回结果',
  other: '协议包装及尚未归类的剩余内容',
};

const EMPTY_BUCKETS: ContextWindowSnapshot['buckets'] = {
  system_prompt: 0,
  runtime: 0,
  chat_history: 0,
  read_file: 0,
  run_command: 0,
  other: 0,
};

const EMPTY_DETAILS: ContextWindowSnapshot['details'] = {
  system_prompt: [{ name: 'system prompts', tokens: 0 }, { name: 'tool definitions', tokens: 0 }],
  runtime: [
    { name: 'runtime state', tokens: 0 },
    { name: 'runtime resources', tokens: 0 },
    { name: 'runtime messages', tokens: 0 },
  ],
  chat_history: [
    { name: 'user messages', tokens: 0 },
    { name: 'assistant messages', tokens: 0 },
    { name: 'other tools', tokens: 0 },
    { name: 'context summary', tokens: 0 },
  ],
  read_file: [
    { name: 'read_ppt', tokens: 0 },
    { name: 'read_image', tokens: 0 },
    { name: 'read_project', tokens: 0 },
  ],
  run_command: [{ name: 'run_command', tokens: 0 }],
  other: [{ name: 'other', tokens: 0 }],
};

const EMPTY_SNAPSHOT: ContextWindowSnapshot = {
  total: 0,
  max: 0,
  ratio: 0,
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

function detailDescription(bucket: ContextBucketKey, name: string): string {
  if (DETAIL_DESCRIPTIONS[name]) return DETAIL_DESCRIPTIONS[name];
  if (bucket === 'run_command') return `${formatDetailName(name)} 命令的调用与返回结果`;
  return '尚未归类的上下文内容';
}

export function ContextWindowPanel() {
  const [open, setOpen] = useState(false);
  const panelId = useId();
  const detailPanelId = `${panelId}-details`;
  const rootRef = useRef<HTMLDivElement>(null);
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
  const compact = useContextWindowStore((state) => state.compact);
  const commitSession = useGitCommitStore((state) => (
    activeProjectId ? state.sessions[activeProjectId] : undefined
  ));
  const briefingActive = useBriefingStore((state) => (
    activeProjectId ? state.sessions[activeProjectId]?.status === 'generating' : false
  ));
  const [activeBucket, setActiveBucket] = useState<ContextBucketKey>('system_prompt');

  useEffect(() => {
    if (threadId && model) void load(threadId, model);
  }, [load, model, threadId]);

  useEffect(() => {
    if (!open) return undefined;
    const closeOnOutsidePress = (event: PointerEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
    };
    const closeOnEscape = (event: globalThis.KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      event.preventDefault();
      const activeElement = document.activeElement;
      if (activeElement instanceof HTMLElement && rootRef.current?.contains(activeElement)) {
        activeElement.blur();
      }
      setOpen(false);
    };
    document.addEventListener('pointerdown', closeOnOutsidePress);
    document.addEventListener('keydown', closeOnEscape);
    return () => {
      document.removeEventListener('pointerdown', closeOnOutsidePress);
      document.removeEventListener('keydown', closeOnEscape);
    };
  }, [open]);

  useEffect(() => {
    setOpen(false);
    setActiveBucket('system_prompt');
  }, [activeProjectId, threadId]);

  const snapshot = session?.snapshot ?? EMPTY_SNAPSHOT;
  const runActive = ['creating', 'running', 'waiting', 'paused', 'recovering', 'canceling'].includes(runStatus);
  const commitActive = commitSession?.status === 'creating' || commitSession?.status === 'running';
  const compacting = session?.compacting || snapshot.status === 'compacting';
  const warning = snapshot.ratio >= 0.8;
  const disabled = !threadId || !model || runActive || commitActive || briefingActive || polishing || compacting;
  const percent = Math.round(snapshot.ratio * 100);
  const details = snapshot.details[activeBucket];
  const activeBucketMeta = BUCKETS.find((bucket) => bucket.key === activeBucket) ?? BUCKETS[0];
  const DetailIcon = activeBucketMeta.icon;

  const segments = useMemo(() => BUCKETS.map((bucket) => ({
    ...bucket,
    width: snapshot.max > 0 ? Math.max(0, snapshot.buckets[bucket.key] / snapshot.max * 100) : 0,
  })), [snapshot]);

  const runCompact = async () => {
    if (!threadId || !model || disabled) return;
    const succeeded = await compact(threadId, model);
    if (!succeeded) return;
    const history = await threadsApi.history(threadId);
    const hydrated = hydrateRunFromHistory(history as unknown as HistoryEntry[]);
    useRunStore.getState().hydrateTimeline(
      threadId, hydrated.items, hydrated.plan, hydrated.session, hydrated.lastEventId,
    );
  };

  return (
    <div ref={rootRef} className="contents">
      <span className="relative inline-flex">
        <IconButton
          label={`上下文窗口，当前使用 ${percent}%`}
          aria-haspopup="dialog"
          aria-expanded={open}
          aria-controls={open ? panelId : undefined}
          onClick={() => setOpen((current) => !current)}
          className={open ? 'bg-panel-muted text-text-900' : undefined}
        >
          <Gauge className="h-4 w-4" strokeWidth={1.75} />
        </IconButton>
        {(warning || compacting) && (
          <span
            aria-hidden="true"
            className={`pointer-events-none absolute bottom-1 right-1 h-1.5 w-1.5 rounded-full ring-2 ring-panel ${
              compacting ? 'bg-success animate-pulse motion-reduce:animate-none' : 'bg-warning'
            }`}
          />
        )}
      </span>

      {open && (
        <section
          id={panelId}
          role="dialog"
          aria-label="上下文窗口"
          className={`absolute right-0 top-[calc(100%+6px)] z-50 w-[min(24rem,calc(100vw-1.5rem))] overflow-hidden rounded-xl border bg-surface shadow-[0_18px_46px_rgba(31,42,55,0.18)] ${
            warning && !compacting ? 'border-warning/50' : 'border-border-strong'
          }`}
        >
          <header className="flex min-h-11 items-center gap-2 px-3 py-2.5">
            <h2 className="text-xs font-semibold text-text-900">上下文窗口</h2>
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
              className="ml-auto inline-flex h-7 items-center gap-1.5 rounded-md border border-border bg-surface px-2.5 text-[11px] font-semibold text-text-600 hover:border-border-strong hover:text-text-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent disabled:cursor-not-allowed disabled:opacity-40"
              title={runActive ? '运行中不可手动压缩' : '手动压缩上下文'}
            >
              {compacting
                ? <Loader2 className="h-3.5 w-3.5 animate-spin motion-reduce:animate-none" />
                : <Minimize2 className="h-3.5 w-3.5" />}
              {compacting ? '压缩中' : '压缩'}
            </button>
          </header>

          <div className="flex items-center gap-2.5 px-3 pb-3">
            <div className="relative flex h-2 min-w-0 flex-1 overflow-hidden rounded-full bg-panel-muted">
              {segments.map((segment) => (
                <span
                  key={segment.key}
                  className="h-full transition-[width] duration-500 ease-out motion-reduce:transition-none"
                  style={{ width: `${segment.width}%`, backgroundColor: segment.color }}
                />
              ))}
              <span className="absolute inset-y-0 left-[85%] w-px bg-danger/70" />
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
            className="context-window-scroll flex gap-1 overflow-x-auto border-t border-border px-2.5 py-2.5"
          >
            {BUCKETS.map((bucket) => (
              <button
                key={bucket.key}
                type="button"
                role="tab"
                aria-selected={activeBucket === bucket.key}
                aria-controls={detailPanelId}
                onClick={() => setActiveBucket(bucket.key)}
                className={`inline-flex shrink-0 items-center gap-1 rounded-md border px-2 py-1 text-[10px] font-semibold ${
                  activeBucket === bucket.key
                    ? 'border-border-strong bg-panel text-text-900'
                    : 'border-transparent text-text-600 hover:bg-panel'
                }`}
              >
                <span className="h-2 w-2 rounded-[3px]" style={{ backgroundColor: bucket.color }} />
                <span className="inline-flex items-baseline gap-1">
                  <span className="leading-none">{bucket.label}</span>
                  <span className="font-mono text-[9px] font-normal leading-none text-text-400 tabular-nums">
                    {formatParentTokens(snapshot.buckets[bucket.key])}
                  </span>
                </span>
              </button>
            ))}
          </div>

          <div
            id={detailPanelId}
            role="tabpanel"
            aria-label={`${activeBucketMeta.label}明细`}
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
                    {detailDescription(activeBucket, detail.name)}
                  </span>
                </span>
                <span className="text-right font-mono text-[10px] leading-4 text-text-600 tabular-nums">
                  {formatDetailTokens(detail.tokens)}
                </span>
              </div>
            ))}
          </div>
        </section>
      )}
    </div>
  );
}
