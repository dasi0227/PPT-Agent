import {
  Check,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  ChevronUp,
  Clipboard,
  FileText,
  Loader2,
  MessageSquarePlus,
  RefreshCw,
  Send,
  X,
} from 'lucide-react';
import { useEffect, useLayoutEffect, useRef, useState } from 'react';
import { useBriefingStore } from '../../stores/briefingStore';
import { useComposerStore } from '../../stores/composerStore';
import { useGitCommitStore } from '../../stores/gitCommitStore';
import { useProjectStore } from '../../stores/projectStore';
import { useThreadStore } from '../../stores/threadStore';
import { useActiveSession } from './useActiveSession';
import type { BriefingTimelineItem } from './eventReducer';
import { MarkdownMessage } from './MarkdownMessage';

const COLLAPSED_HEIGHT = 300;

function BriefingLoading({ item }: { item: BriefingTimelineItem }) {
  const activeProjectId = useProjectStore((state) => state.activeProjectId);
  const cancel = useBriefingStore((state) => state.cancel);
  const [elapsed, setElapsed] = useState(0);
  const startedAt = item.loadingStartedAt ?? Date.now();
  useEffect(() => {
    const update = () => setElapsed(Math.max(0, Math.floor((Date.now() - startedAt) / 1000)));
    update();
    const timer = window.setInterval(update, 1000);
    return () => window.clearInterval(timer);
  }, [startedAt]);
  const progress = Math.min(92, Math.round((elapsed / 45) * 92));
  return (
    <article className="rounded-lg border border-border bg-surface px-3 py-3" aria-live="polite">
      <div className="flex items-center gap-2">
        <span className="grid h-7 w-7 place-items-center rounded-full bg-accent-soft text-accent">
          <Loader2 className="h-3.5 w-3.5 animate-spin motion-reduce:animate-none" strokeWidth={1.75} />
        </span>
        <span className="min-w-0 flex-1">
          <span className="block text-xs font-semibold text-text-900">
            正在生成{item.kind === 'kickoff' ? '启动简报' : '交接简报'}
          </span>
          <span className="block text-[11px] tabular-nums text-text-400">整理项目上下文 · {elapsed}s</span>
        </span>
        <button
          type="button"
          onClick={() => activeProjectId && cancel(activeProjectId)}
          className="grid h-7 w-7 place-items-center rounded-md text-text-400 hover:bg-panel-muted hover:text-text-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
          aria-label="取消生成"
          title="取消生成"
        >
          <X className="h-3.5 w-3.5" strokeWidth={1.75} />
        </button>
      </div>
      <div className="mt-3 h-1 overflow-hidden rounded-full bg-panel-muted" aria-hidden="true">
        <span className="block h-full bg-accent transition-[width] duration-1000" style={{ width: `${progress}%` }} />
      </div>
      <div className="mt-3 space-y-2" aria-hidden="true">
        <span className="block h-2.5 w-[92%] animate-pulse rounded-sm bg-panel-muted motion-reduce:animate-none" />
        <span className="block h-2.5 w-[78%] animate-pulse rounded-sm bg-panel-muted motion-reduce:animate-none" />
        <span className="block h-2.5 w-[86%] animate-pulse rounded-sm bg-panel-muted motion-reduce:animate-none" />
      </div>
    </article>
  );
}

export function BriefingActivity({ item }: { item: BriefingTimelineItem }) {
  const activeProjectId = useProjectStore((state) => state.activeProjectId);
  const model = useComposerStore((state) => state.modelProfileName);
  const polishing = useComposerStore((state) => state.polishing);
  const commitSession = useGitCommitStore((state) => (
    activeProjectId ? state.sessions[activeProjectId] : undefined
  ));
  const generate = useBriefingStore((state) => state.generate);
  const createThread = useThreadStore((state) => state.createThread);
  const setThreadDraft = useComposerStore((state) => state.setThreadDraft);
  const briefingSession = useBriefingStore((state) => (
    activeProjectId ? state.sessions[activeProjectId] : undefined
  ));
  const { status: runStatus } = useActiveSession();
  const [versionIndex, setVersionIndex] = useState(Math.max(0, item.versions.length - 1));
  const [expanded, setExpanded] = useState(false);
  const [overflowing, setOverflowing] = useState(false);
  const [feedbackOpen, setFeedbackOpen] = useState(false);
  const [feedback, setFeedback] = useState('');
  const [copied, setCopied] = useState(false);
  const [creatingContinuation, setCreatingContinuation] = useState(false);
  const contentRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    setVersionIndex(Math.max(0, item.versions.length - 1));
  }, [item.versions.length]);
  const version = item.versions[versionIndex];

  useLayoutEffect(() => {
    const content = contentRef.current;
    if (!content) return undefined;
    const measure = () => setOverflowing(content.scrollHeight > COLLAPSED_HEIGHT + 1);
    measure();
    const observer = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(measure);
    observer?.observe(content);
    return () => observer?.disconnect();
  }, [version?.content]);

  if (item.status === 'loading') return <BriefingLoading item={item} />;
  if (!version) return null;

  const latestIndex = item.versions.length - 1;
  const isLatest = versionIndex === latestIndex;
  const runActive = ['creating', 'running', 'waiting', 'paused', 'recovering', 'canceling'].includes(runStatus);
  const commitActive = commitSession?.status === 'creating' || commitSession?.status === 'running';
  const busy = runActive || commitActive || polishing || briefingSession?.status === 'generating';

  const copy = async () => {
    await navigator.clipboard?.writeText(version.content);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1200);
  };
  const retry = async () => {
    const trimmed = feedback.trim();
    if (!trimmed || !activeProjectId || !model || busy) return;
    const succeeded = await generate(
      activeProjectId,
      version.thread_id,
      model,
      item.kind,
      item.briefingId,
      trimmed,
    );
    if (succeeded) {
      setFeedback('');
      setFeedbackOpen(false);
      setExpanded(false);
    }
  };
  const createContinuation = async () => {
    if (!activeProjectId || creatingContinuation) return;
    setCreatingContinuation(true);
    try {
      const threadId = await createThread(activeProjectId);
      setThreadDraft(threadId, version.content);
    } finally {
      setCreatingContinuation(false);
    }
  };

  return (
    <article className="overflow-hidden rounded-lg border border-border bg-surface">
      <header className="flex min-h-10 items-center gap-2 border-b border-border px-3 py-2">
        <span className="grid h-6 w-6 place-items-center rounded-full bg-accent-soft text-accent">
          <FileText className="h-3.5 w-3.5" strokeWidth={1.75} />
        </span>
        <span className="text-xs font-semibold text-text-900">
          {item.kind === 'kickoff' ? 'Kickoff' : 'Handoff'}
        </span>
        <div className="ml-auto flex items-center gap-0.5">
          {item.versions.length > 1 && (
            <div className="mr-1 inline-flex h-7 items-center" aria-label="简报版本">
              <button
                type="button"
                onClick={() => setVersionIndex((index) => Math.max(0, index - 1))}
                disabled={versionIndex === 0}
                className="grid h-7 w-6 place-items-center rounded-md text-text-400 hover:bg-panel-muted hover:text-text-900 disabled:opacity-30"
                aria-label="上一版"
              >
                <ChevronLeft className="h-3.5 w-3.5" />
              </button>
              <span className="min-w-10 text-center font-mono text-[10px] tabular-nums text-text-500">
                {versionIndex + 1} / {item.versions.length}
              </span>
              <button
                type="button"
                onClick={() => setVersionIndex((index) => Math.min(latestIndex, index + 1))}
                disabled={isLatest}
                className="grid h-7 w-6 place-items-center rounded-md text-text-400 hover:bg-panel-muted hover:text-text-900 disabled:opacity-30"
                aria-label="下一版"
              >
                <ChevronRight className="h-3.5 w-3.5" />
              </button>
            </div>
          )}
          {isLatest && (
            <button
              type="button"
              onClick={() => setFeedbackOpen((open) => !open)}
              disabled={busy}
              className="grid h-7 w-7 place-items-center rounded-md text-text-500 hover:bg-panel-muted hover:text-accent disabled:cursor-not-allowed disabled:opacity-35"
              aria-label="重新生成"
              title="重新生成"
            >
              <RefreshCw className="h-3.5 w-3.5" strokeWidth={1.75} />
            </button>
          )}
          <button
            type="button"
            onClick={() => void copy()}
            className="grid h-7 w-7 place-items-center rounded-md text-text-500 hover:bg-panel-muted hover:text-text-900"
            aria-label="复制全文"
            title={copied ? '已复制' : '复制全文'}
          >
            {copied ? <Check className="h-3.5 w-3.5 text-success" /> : <Clipboard className="h-3.5 w-3.5" />}
          </button>
        </div>
      </header>
      <div className="relative">
        <div
          ref={contentRef}
          className="px-3 py-3 text-sm leading-[1.65] text-text-900"
          style={expanded ? undefined : { maxHeight: COLLAPSED_HEIGHT, overflow: 'hidden' }}
        >
          <MarkdownMessage content={version.content} />
        </div>
        {overflowing && !expanded && (
          <div className="pointer-events-none absolute inset-x-0 bottom-0 h-20 bg-gradient-to-b from-transparent to-surface" />
        )}
        {overflowing && (
          <button
            type="button"
            onClick={() => setExpanded((value) => !value)}
            className="absolute bottom-2 left-1/2 inline-flex h-7 -translate-x-1/2 items-center gap-1 rounded-full border border-border bg-surface px-2.5 text-[11px] text-text-600 shadow-sm hover:text-text-900"
            aria-expanded={expanded}
          >
            {expanded ? <ChevronUp className="h-3.5 w-3.5" /> : <ChevronDown className="h-3.5 w-3.5" />}
            {expanded ? '收起' : '展开全文'}
          </button>
        )}
      </div>
      <div className="border-t border-border px-3 py-2">
        <button
          type="button"
          onClick={() => void createContinuation()}
          disabled={creatingContinuation}
          className="inline-flex h-8 w-full items-center justify-center gap-1.5 rounded-md bg-accent px-3 text-xs font-semibold text-white transition-colors hover:bg-accent/90 disabled:cursor-not-allowed disabled:bg-text-400 disabled:opacity-55"
        >
          {creatingContinuation
            ? <Loader2 className="h-3.5 w-3.5 animate-spin motion-reduce:animate-none" strokeWidth={1.75} />
            : <MessageSquarePlus className="h-3.5 w-3.5" strokeWidth={1.75} />}
          以此版本新建会话
        </button>
      </div>
      {feedbackOpen && isLatest && (
        <div className="border-t border-border px-3 py-3">
          <label htmlFor={`briefing-feedback-${item.briefingId}`} className="mb-1.5 block text-[11px] font-semibold text-text-600">
            调整建议
          </label>
          <textarea
            id={`briefing-feedback-${item.briefingId}`}
            value={feedback}
            onChange={(event) => setFeedback(event.target.value)}
            rows={3}
            maxLength={4000}
            placeholder="说明需要补充、删减或调整的内容"
            className="block w-full resize-none rounded-md border border-border bg-panel px-2.5 py-2 text-xs leading-5 text-text-900 placeholder:text-text-400 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
          />
          <div className="mt-2 flex justify-end gap-1">
            <button
              type="button"
              onClick={() => {
                setFeedbackOpen(false);
                setFeedback('');
              }}
              className="inline-flex h-7 items-center gap-1 rounded-md px-2 text-xs text-text-500 hover:bg-panel-muted hover:text-text-900"
            >
              <X className="h-3.5 w-3.5" />
              取消
            </button>
            <button
              type="button"
              onClick={() => void retry()}
              disabled={!feedback.trim() || busy || !model}
              className="inline-flex h-7 items-center gap-1 rounded-md bg-accent px-2.5 text-xs text-white disabled:cursor-not-allowed disabled:opacity-45"
            >
              <Send className="h-3.5 w-3.5" />
              提交
            </button>
          </div>
        </div>
      )}
    </article>
  );
}
