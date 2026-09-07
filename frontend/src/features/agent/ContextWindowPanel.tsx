import {
  Ellipsis,
  FileText,
  Gauge,
  History,
  Loader2,
  MessageSquare,
  Minimize2,
  Shield,
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
  { key: 'read_ppt', label: '读文件', color: '#2F67F6', icon: FileText },
  { key: 'run_command', label: '跑命令', color: '#0D9488', icon: Terminal },
  { key: 'system_prompt', label: '系统提示词', color: '#7C3AED', icon: Shield },
  { key: 'user_prompt', label: '用户提示词', color: '#D97706', icon: MessageSquare },
  { key: 'chat_history', label: '对话历史', color: '#DB2777', icon: History },
  { key: 'other', label: '其它', color: '#8793A2', icon: Ellipsis },
];

const EMPTY_BUCKETS: ContextWindowSnapshot['buckets'] = {
  read_ppt: 0, run_command: 0, system_prompt: 0,
  user_prompt: 0, chat_history: 0, other: 0,
};
const EMPTY_DETAILS: ContextWindowSnapshot['details'] = {
  read_ppt: [], run_command: [], system_prompt: [],
  user_prompt: [], chat_history: [], other: [],
};
const EMPTY_SNAPSHOT: ContextWindowSnapshot = {
  total: 0,
  max: 0,
  ratio: 0,
  status: 'idle',
  buckets: EMPTY_BUCKETS,
  details: EMPTY_DETAILS,
};

function formatTokens(tokens: number): string {
  if (tokens >= 1000) return `${(tokens / 1000).toFixed(1)}k`;
  return String(tokens);
}

export function ContextWindowPanel() {
  const [open, setOpen] = useState(false);
  const panelId = useId();
  const rootRef = useRef<HTMLDivElement>(null);
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
  const compact = useContextWindowStore((state) => state.compact);
  const commitSession = useGitCommitStore((state) => (
    activeProjectId ? state.sessions[activeProjectId] : undefined
  ));
  const briefingActive = useBriefingStore((state) => (
    activeProjectId ? state.sessions[activeProjectId]?.status === 'generating' : false
  ));
  const [activeBucket, setActiveBucket] = useState<ContextBucketKey>('read_ppt');

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
      setOpen(false);
      triggerRef.current?.focus();
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
  }, [activeProjectId, threadId]);

  const snapshot = session?.snapshot ?? EMPTY_SNAPSHOT;
  const runActive = ['creating', 'running', 'waiting', 'paused', 'recovering', 'canceling'].includes(runStatus);
  const commitActive = commitSession?.status === 'creating' || commitSession?.status === 'running';
  const compacting = session?.compacting || snapshot.status === 'compacting';
  const status = compacting
    ? 'compacting'
    : runActive
      ? (snapshot.ratio >= 0.8 ? 'warning' : 'running')
      : 'idle';
  const disabled = !threadId || !model || runActive || commitActive || briefingActive || polishing || compacting;
  const percent = Math.round(snapshot.ratio * 100);
  const details = snapshot.details[activeBucket] ?? [];
  const statusMeta = {
    idle: { label: '空闲', classes: 'bg-panel-muted text-text-600', dot: 'bg-text-400' },
    running: { label: '运行中', classes: 'bg-accent-soft text-accent', dot: 'bg-accent animate-pulse motion-reduce:animate-none' },
    warning: { label: '接近阈值', classes: 'bg-warning-soft text-warning', dot: 'bg-warning animate-pulse motion-reduce:animate-none' },
    compacting: { label: '压缩中', classes: 'bg-success-soft text-success', dot: 'bg-success animate-pulse motion-reduce:animate-none' },
  }[status];

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
          ref={triggerRef}
          label={`上下文窗口，当前使用 ${percent}%`}
          aria-haspopup="dialog"
          aria-expanded={open}
          aria-controls={open ? panelId : undefined}
          onClick={() => setOpen((current) => !current)}
          className={open ? 'bg-panel-muted text-text-900' : undefined}
        >
          <Gauge className="h-4 w-4" strokeWidth={1.75} />
        </IconButton>
        {(status === 'warning' || status === 'compacting') && (
          <span
            aria-hidden="true"
            className={`pointer-events-none absolute bottom-1 right-1 h-1.5 w-1.5 rounded-full ring-2 ring-panel ${
              status === 'warning' ? 'bg-warning' : 'bg-success animate-pulse motion-reduce:animate-none'
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
            status === 'warning' ? 'border-warning/50' : 'border-border-strong'
          }`}
        >
          <header className="flex min-h-11 items-center gap-2 px-3 py-2.5">
            <h2 className="text-xs font-semibold text-text-900">上下文窗口</h2>
            <span className={`inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-[10px] font-semibold ${statusMeta.classes}`}>
              <span className={`h-1.5 w-1.5 rounded-full ${statusMeta.dot}`} />
              {statusMeta.label}
            </span>
            {session?.loading && <Loader2 className="h-3.5 w-3.5 animate-spin text-text-400 motion-reduce:animate-none" />}
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
                  className="h-full transition-[width] duration-500 ease-out"
                  style={{ width: `${segment.width}%`, backgroundColor: segment.color }}
                />
              ))}
              <span className="absolute inset-y-0 left-[85%] w-px bg-danger/70" />
            </div>
            <span className={`min-w-10 text-right font-mono text-xs font-semibold tabular-nums ${
              status === 'warning' ? 'text-warning' : status === 'compacting' ? 'text-success' : 'text-text-600'
            }`}>
              {percent}%
            </span>
          </div>

          <div className="context-window-scroll flex gap-1 overflow-x-auto border-t border-border px-2.5 py-2.5">
            {BUCKETS.map((bucket) => (
              <button
                key={bucket.key}
                type="button"
                onClick={() => setActiveBucket(bucket.key)}
                className={`inline-flex shrink-0 items-center gap-1 rounded-md border px-2 py-1 text-[10px] font-semibold ${
                  activeBucket === bucket.key
                    ? 'border-border-strong bg-panel text-text-900'
                    : 'border-transparent text-text-600 hover:bg-panel'
                }`}
              >
                <span className="h-2 w-2 rounded-[3px]" style={{ backgroundColor: bucket.color }} />
                {bucket.label}
                <span className="font-mono text-[9px] font-normal text-text-400">
                  {formatTokens(snapshot.buckets[bucket.key])}
                </span>
              </button>
            ))}
          </div>

          {details.length > 0 && (
            <div className="max-h-48 overflow-y-auto border-t border-border">
              {details.map((detail, index) => {
                const bucket = BUCKETS.find((candidate) => candidate.key === activeBucket) ?? BUCKETS[0];
                const Icon = bucket.icon;
                return (
                  <div key={`${detail.name}:${index}`} className="flex items-center gap-2 border-b border-border px-3 py-2 last:border-b-0">
                    <span className="grid h-6 w-6 shrink-0 place-items-center rounded-md bg-panel" style={{ color: bucket.color }}>
                      <Icon className="h-3.5 w-3.5" strokeWidth={1.75} />
                    </span>
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-[11px] font-semibold text-text-900">{detail.name}</span>
                      <span className="block truncate text-[9px] text-text-400">{detail.source}</span>
                    </span>
                    <span className={`rounded px-1.5 py-0.5 text-[8px] font-bold ${
                      detail.layer === 'transcript' ? 'bg-accent-soft text-accent' : 'bg-panel-muted text-text-600'
                    }`}>
                      {detail.layer === 'transcript' ? 'TRANSCRIPT' : 'SEED'}
                    </span>
                    <span className="min-w-10 text-right font-mono text-[10px] text-text-600">
                      {formatTokens(detail.tokens)}
                    </span>
                  </div>
                );
              })}
            </div>
          )}
        </section>
      )}
    </div>
  );
}
