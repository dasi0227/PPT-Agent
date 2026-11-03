import { AgentCommandRow } from './AgentCommandRow';
import { TextCommandActivity } from './TextCommandActivity';
import { RollbackButton } from './ProjectHistoryControls';
import React, { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { AlertTriangle, ArrowDown, ChevronRight, Code2, FileImage, PauseCircle, StopCircle, XCircle, Loader2, RotateCcw, ArrowUp } from 'lucide-react';
import { targetLabel } from './runtimeLabels';
import { useActiveSession, useActiveThreadId } from './useActiveSession';
import { FinalMessage } from './FinalMessage';
import { LiveProgressRow } from './LiveProgressRow';
import { MarkdownMessage } from './MarkdownMessage';
import { QuestionPanel } from './QuestionPanel';
import { ThinkingRow, MilestoneRow, ToolActivityRow, ToolGroupRow } from './ActivityRows';
import { TerminalNotice } from './TerminalNotice';
import { PlanApproval } from './PlanApproval';
import { MessageMetaActions } from './MessageMetaActions';
import type { TimelineItem } from './timelineTypes';
import { DisplayEntry, groupTimelineItems, visibleTimelineItems } from './timelineGrouping';
import { useAgentSettingsStore } from '../../stores/agentSettingsStore';
import { TimelineDisclosure } from './TimelineDisclosure';
import { GitCommitEvent, GitCommitProgress } from './GitCommitActivity';
import { useProjectStore } from '../../stores/projectStore';
import { useThreadStore } from '../../stores/threadStore';
import { useGitCommitStore } from '../../stores/gitCommitStore';
import { BriefingActivity } from './BriefingActivity';
import { ContextCompactionActivity } from './ContextCompactionActivity';
import { ScopeExpansionCard } from './ScopeExpansionCard';
import { useSessionStore } from '../../stores/sessionStore';
import { compareRevision } from '../../api/session';
import { attachmentsApi } from '../../api/attachments';
import { ImagePreview } from '../../components/ui/ImagePreview';
import { runElapsed, type RunClock } from './runClock';
import { formatRunTokenUsage, type RunTokenUsage } from './runTokenUsage';
import { isResourceEditTool } from '../../api/resourceTools';
import { EmptyTimeline } from './EmptyTimeline';
import type { StarterTemplate } from './starterTemplates';

const messageReferenceClassName = 'inline-flex h-7 shrink-0 items-center gap-1 rounded-md border border-border bg-surface px-2 text-[11px] font-semibold text-text-900 ui-interactive';
const runHeaderClassName = 'flex min-h-8 w-full items-center gap-2 px-1.5 py-1 text-left text-[13px] text-text-600';
const timelineTopPadding = 12;

function formatDuration(durationMs?: number): string {
  if (durationMs === undefined || !Number.isFinite(durationMs) || durationMs < 0) return '--';
  const totalSeconds = Math.max(0, Math.round(durationMs / 1000));
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  if (minutes <= 0) return `${seconds}s`;
  return `${minutes}m ${seconds}s`;
}

const runSummaryLabel = {
  completed: 'Dasi 执行完成',
  canceled: 'Dasi 执行中断',
  paused: 'Dasi 执行中断',
  failed: 'Dasi 执行失败',
  error: 'Dasi 执行失败',
} as const;

function RunLogo({ src = '/logo.png' }: { src?: string }) {
  return <img src={src} alt="" aria-hidden="true" width={24} height={24} className="h-6 w-6 shrink-0 object-contain" />;
}

function RunningRunHeader({ clock, logoSrc, usage, waiting }: { clock: RunClock; logoSrc: string; usage?: RunTokenUsage; waiting: boolean }) {
  const [now, setNow] = useState(Date.now);
  const tokens = formatRunTokenUsage(usage);
  useEffect(() => {
    setNow(Date.now());
    if (clock.runningSince === null) return;
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, [clock.runningSince]);
  return (
    <div className="pb-1.5">
      <div className={runHeaderClassName}>
        <RunLogo src={logoSrc} />
        <span className="min-w-0 flex-1 truncate leading-5">Dasi {waiting ? '等待回答' : '正在执行'}，已进行 {formatDuration(runElapsed(clock, now))}{tokens !== null && `，已耗费 ${tokens} Tokens`}</span>
      </div>
      <div className="mt-1.5 border-t border-border" />
    </div>
  );
}

function RunStatusIcon({ status }: { status: 'completed' | 'failed' | 'error' | 'canceled' | 'paused' }) {
  if (status === 'completed') {
    return <RunLogo />;
  }
  if (status === 'paused') {
    return <PauseCircle className="h-4 w-4 shrink-0 text-text-400" strokeWidth={1.75} />;
  }
  if (status === 'canceled') {
    return <StopCircle className="h-4 w-4 shrink-0 text-danger" strokeWidth={1.75} />;
  }
  if (status === 'error') {
    return <AlertTriangle className="h-4 w-4 shrink-0 text-warning" strokeWidth={1.75} />;
  }
  return <XCircle className="h-4 w-4 shrink-0 text-danger" strokeWidth={1.75} />;
}

export const Timeline: React.FC<{
  starterAvailable?: boolean;
  onStarterSelect?: (template: StarterTemplate) => void;
}> = ({ starterAvailable = false, onStarterSelect }) => {
  const session = useActiveSession();
  const threadId = useActiveThreadId();
  const { activeRunId, timelineItems, status, plan, progress } = session;
  const showToolFailures = useAgentSettingsStore(state => state.value?.show_tool_failures ?? false);
  const sync = useSessionStore(state => threadId ? state.sessions[threadId] : undefined);
  const activeProjectId = useProjectStore((state) => state.activeProjectId);
  const commitSession = useGitCommitStore((state) => (
    activeProjectId ? state.sessions[activeProjectId] : undefined
  ));
  const containerRef = useRef<HTMLDivElement>(null);
  const latestTurnRef = useRef<HTMLDivElement>(null);
  const followingRef = useRef(true);
  const anchorTopRef = useRef<number | null>(null);
  const viewportSizeRef = useRef<{ width: number; height: number } | null>(null);
  const alignedTurnRef = useRef<string | null>(null);
  const pageAnchor = useRef<{ key: string; top: number; groupId?: string } | null>(null);
  const displayedSnapshot = useRef<string | null>(null);
  const previousTurnRef = useRef<{ threadId: string | null; id: string | null } | null>(null);
  const [showReturn, setShowReturn] = useState(false);
  const [anchoredTurnId, setAnchoredTurnId] = useState<string | null>(null);
	const [openDOMReference, setOpenDOMReference] = useState<{ key:string; comment:string; status:string } | null>(null);
  const reducedMotion = typeof window !== 'undefined'
    && typeof window.matchMedia === 'function'
    && window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  const displayEntries = useMemo(
    () => groupTimelineItems(visibleTimelineItems(timelineItems, showToolFailures), session.groups),
    [timelineItems, session.groups, showToolFailures],
  );
  const latestTurnIndex = displayEntries.reduce((index, entry, entryIndex) => (
    entry.kind === 'item' && entry.item.type === 'user_turn' ? entryIndex : index
  ), -1);
  const latestTurnEntry = latestTurnIndex < 0 ? null : displayEntries[latestTurnIndex];
  const latestTurnId = latestTurnEntry?.kind === 'item' ? latestTurnEntry.item.id : null;
  const runningTurn = ['creating', 'running', 'waiting', 'recovering', 'canceling'].includes(status)
    ? status === 'creating'
      ? timelineItems.find((item) => item.id === latestTurnId)
      : timelineItems.find((item) => item.type === 'user_turn' && item.runId === activeRunId)
    : undefined;
  const runningLogoSrc = useMemo(() => {
    if (session.mode !== 'create') return '/dasi-B.png';
    if (activeRunId) {
      // Items retain their tool-start order when completion events update them.
      for (let index = timelineItems.length - 1; index >= 0; index -= 1) {
        const item = timelineItems[index];
        if (item.type !== 'tool' || item.runId !== activeRunId) continue;
        if (item.tool === 'render_slide') return '/dasi-D.png';
        if (isResourceEditTool(item.tool)) return '/dasi-C.png';
      }
    }
    return '/dasi-C.png';
  }, [activeRunId, session.mode, timelineItems]);
  const earlierEntries = latestTurnIndex < 0 ? [] : displayEntries.slice(0, latestTurnIndex);
  const latestEntries = latestTurnIndex < 0 ? displayEntries : displayEntries.slice(latestTurnIndex);
  const commitActive = commitSession?.sourceThreadId===threadId && (commitSession?.status === 'creating' || commitSession?.status === 'running');
  const showEmptyWordmark = session.loadStatus === 'ready' && session.isEmpty && !commitActive;
  const initialLoading = ['uninitialized', 'loading', 'refreshing'].includes(session.loadStatus) && timelineItems.length === 0;
  const visibleProgress = progress ?? (activeRunId
    ? status === 'recovering' ? { activity: 'run.recovering' as const }
      : status === 'running' ? { activity: 'run.analyzing' as const } : null
    : null);

  const scrollToLatest = useCallback((smooth: boolean) => {
    const container = containerRef.current;
    if (!container) return;
    if (typeof container.scrollTo === 'function') {
      container.scrollTo({
        top: container.scrollHeight,
        behavior: smooth && !reducedMotion ? 'smooth' : 'auto',
      });
    } else {
      container.scrollTop = container.scrollHeight;
    }
    followingRef.current = true;
    anchorTopRef.current = null;
    setShowReturn(false);
  }, [reducedMotion]);

  const alignLatestTurn = useCallback(() => {
    const container = containerRef.current;
    const turn = latestTurnRef.current;
    if (!container || !turn) return;
    const targetTop = turn.getBoundingClientRect().top
      - container.getBoundingClientRect().top + container.scrollTop;
    container.scrollTop = Math.max(0, targetTop - timelineTopPadding);
    anchorTopRef.current = container.scrollTop;
    followingRef.current = false;
    setShowReturn(false);
  }, []);

  useLayoutEffect(() => {
    const previous = previousTurnRef.current;
    previousTurnRef.current = { threadId, id: latestTurnId };
    if (!previous || previous.threadId !== threadId) {
      alignedTurnRef.current = null;
      anchorTopRef.current = null;
      followingRef.current = true;
      setAnchoredTurnId(status === 'creating' && latestTurnId?.startsWith('pending:') ? latestTurnId : null);
      return;
    }
    const latest = timelineItems.find(item => item.id === latestTurnId);
    const newInput = latest?.id.startsWith('pending:') || Boolean(latest?.createdSeq && sync?.snapshotSeq
      && compareRevision(latest.createdSeq, sync.snapshotSeq) > 0);
    if (newInput && latestTurnId && latestTurnId !== previous.id) {
      alignedTurnRef.current = null;
      followingRef.current = false;
      setAnchoredTurnId(latestTurnId);
    }
  }, [latestTurnId, status, threadId, timelineItems, sync?.snapshotSeq]);

  useLayoutEffect(() => {
    const container = containerRef.current;
    if (pageAnchor.current) {
      const anchor = pageAnchor.current;
      if ((anchor.groupId ? sync?.groupPages[anchor.groupId]?.status : sync?.historyPage.status) === 'loading') return;
      const node = Array.from(container?.querySelectorAll<HTMLElement>('[data-timeline-key]') ?? []).find(value => value.dataset.timelineKey === anchor.key);
      if (container && node) container.scrollTop += node.getBoundingClientRect().top - anchor.top;
      pageAnchor.current = null;
      return;
    }
    if (anchoredTurnId && anchoredTurnId === latestTurnId && latestTurnRef.current
      && alignedTurnRef.current !== `${threadId}:${anchoredTurnId}` && container) {
      alignLatestTurn();
      alignedTurnRef.current = `${threadId}:${anchoredTurnId}`;
      return;
    }
    const snapshotKey = sync?.snapshotSeq ? `${threadId}:${sync.generation}:${sync.snapshotSeq}` : null;
    const firstSnapshot = snapshotKey !== displayedSnapshot.current;
    displayedSnapshot.current = snapshotKey;
    if (followingRef.current) scrollToLatest(!firstSnapshot);
    else if (container) setShowReturn(container.scrollHeight - container.scrollTop - container.clientHeight > 80);
  }, [alignLatestTurn, anchoredTurnId, latestTurnId, plan, scrollToLatest, threadId, timelineItems, sync?.historyPage.status, sync?.groupPages, sync?.generation, sync?.snapshotSeq]);

  useLayoutEffect(() => {
    const container = containerRef.current;
    if (!container || typeof ResizeObserver === 'undefined') return;
    viewportSizeRef.current = { width: container.clientWidth, height: container.clientHeight };
    const observer = new ResizeObserver(() => {
      const previous = viewportSizeRef.current;
      const size = { width: container.clientWidth, height: container.clientHeight };
      viewportSizeRef.current = size;
      if (previous && previous.width === size.width && previous.height === size.height) return;
      if (anchorTopRef.current !== null) alignLatestTurn();
      else if (followingRef.current) scrollToLatest(false);
      else setShowReturn(container.scrollHeight - container.scrollTop - container.clientHeight > 80);
    });
    observer.observe(container);
    return () => {
      observer.disconnect();
      viewportSizeRef.current = null;
    };
  }, [alignLatestTurn, scrollToLatest, threadId]);

  useEffect(() => {
    if (followingRef.current && progress) scrollToLatest(false);
  }, [progress, scrollToLatest]);

  const handleScroll = () => {
    const container = containerRef.current;
    if (!container) return;
    // Resizing can clamp scrollTop before ResizeObserver restores the active positioning mode.
    const size = viewportSizeRef.current;
    if (size && (size.width !== container.clientWidth || size.height !== container.clientHeight)) return;
    if (anchorTopRef.current !== null && Math.abs(container.scrollTop - anchorTopRef.current) <= 1) return;
    anchorTopRef.current = null;
    const nearBottom = container.scrollHeight - container.scrollTop - container.clientHeight <= 80;
    followingRef.current = nearBottom;
    setShowReturn(!nearBottom);
  };

  const keepPageAnchor = (groupId?: string) => {
    const container = containerRef.current;
    if (!container) return;
    const top = container.getBoundingClientRect().top;
    const node = Array.from(container.querySelectorAll<HTMLElement>('[data-timeline-key]')).find(value => value.getBoundingClientRect().bottom >= top);
    if (node?.dataset.timelineKey) pageAnchor.current = { key: node.dataset.timelineKey, top: node.getBoundingClientRect().top, groupId };
    followingRef.current = false;
    anchorTopRef.current = null;
  };
  const loadOlder = () => {
    if (!threadId || sync?.historyPage.status === 'loading') return;
    keepPageAnchor();
    void useSessionStore.getState().loadOlder(threadId);
  };

  const loadGroup = (groupId: string, preserveAnchor = true) => {
    if (!threadId || sync?.groupPages[groupId]?.status === 'loading') return;
    if (preserveAnchor) keepPageAnchor(groupId);
    void useSessionStore.getState().loadGroup(threadId, groupId);
  };

  const renderItem = (item: TimelineItem, animateEntry = true) => {
    return (
      <div key={item.id} data-timeline-key={item.id} className={animateEntry ? 'motion-safe:animate-[timeline-enter_120ms_ease-out]' : undefined}>
        {item.type === 'user_turn' && (
          <>
            <div className="flex justify-end">
              <div className="group flex max-w-[88%] flex-col items-end">
                <div className="rounded-[10px] bg-panel-muted px-3 py-2">
				  {item.referenceOrder && item.referenceOrder.length > 0 && (
					<div className="scrollbar-none mb-2 flex max-w-full gap-1.5 overflow-x-auto" aria-label="消息引用">
					  {item.referenceOrder.map((reference) => {
						if (reference.kind === 'image') {
                          const projectId = session.projectId ?? activeProjectId;
                          if (!projectId) return null;
                          return (
                            <ImagePreview
                              key={`${projectId}:${threadId}:image:${reference.ref_id}`}
                              name="图片"
                              src={attachmentsApi.contentUrl(projectId, reference.ref_id, 'original')}
                              className={messageReferenceClassName}
                            >
                              <FileImage className="h-3.5 w-3.5 text-accent" aria-hidden="true" />图片
                            </ImagePreview>
                          );
                        }
						const selection = item.domSelections?.find((candidate) => candidate.selection_id === reference.ref_id);
						if (!selection) return null;
						const key = `${item.id}:${selection.selection_id}`;
						return <button key={key} type="button" onClick={() => setOpenDOMReference(openDOMReference?.key === key ? null : { key, comment:selection.comment, status:selection.status })} className={messageReferenceClassName}><Code2 className="h-3.5 w-3.5 text-accent" />标记 {selection.marker_no}</button>;
					  })}
					</div>
				  )}
				  {openDOMReference?.key.startsWith(`${item.id}:`) && <div className="mb-2 rounded-md border border-border bg-surface p-2 text-xs text-text-600"><div className="whitespace-pre-wrap">{openDOMReference.comment || '未填写注释'}</div>{openDOMReference.status !== 'active' && <div className="mt-1 text-warning">{openDOMReference.status === 'page_deleted' ? '页面已删除' : '内容已删除'}</div>}</div>}
                  <MarkdownMessage content={item.text} className="agent-user-markdown [&>:first-child]:mt-0 [&>:last-child]:mb-0" />
                  {item.deliveryStatus && item.deliveryStatus !== 'accepted' && (
                    <div className={`mt-1 text-[10px] ${
                      item.deliveryStatus === 'rejected' ? 'text-danger' : 'text-text-400'
                    }`}>
                      {item.deliveryStatus === 'sending'
                        ? '发送中'
                        : '未能加入当前任务'}
                    </div>
                  )}
                </div>
                <MessageMetaActions
                  text={item.text}
                  timestamp={item.timestamp}
                  label="复制用户消息"
                  scopeLabel={item.scope ? targetLabel(item.scope) : undefined}
                ><RollbackButton runId={item.runId} steering={item.steering} /></MessageMetaActions>
              </div>
            </div>

          </>
        )}
        {item.type === 'thinking' && <ThinkingRow item={item} />}
        {item.type === 'milestone' && <MilestoneRow item={item} />}
        {item.type === 'tool' && <ToolActivityRow item={item} />}
        {item.type === 'question' && <QuestionPanel item={item} />}
        {item.type === 'plan_approval' && <PlanApproval item={item} />}
        {item.type === 'scope_expansion' && <ScopeExpansionCard item={item} />}
        {item.type === 'final' && <FinalMessage item={item} />}
        {item.type === 'input_reply' && <FinalMessage item={{ ...item, type: 'final' }} />}
        {item.type === 'terminal_notice' && <TerminalNotice item={item} />}
        {item.type === 'git_commit' && !(item.status === 'loading' && commitActive && commitSession?.operationId === item.operationId) && <GitCommitEvent item={item} />}
        {item.type === 'command' && (item.commandSource === 'automatic' && (item.kind === 'rename' || item.kind === 'compact') ? <AgentCommandRow item={item} /> : <TextCommandActivity item={item} />)}
        {item.type === 'briefing' && <BriefingActivity item={item} />}
        {item.type === 'context_compaction' && (item.trigger === 'auto' ? <AgentCommandRow item={item} /> : <ContextCompactionActivity item={item} />)}
        {item.detailRef && threadId && <TimelineDetail threadId={threadId} item={item} />}
      </div>
    );
  };

  const renderEntry = (entry: DisplayEntry, animateEntry = true): React.ReactNode => {
    if (entry.kind === 'group_page') return <GroupPageButton key={entry.id} threadId={threadId} groupId={entry.groupId} onLoad={() => loadGroup(entry.groupId)} />;
    const created = entry.kind === 'item' ? entry.item.createdSeq : entry.kind === 'tool_group' ? entry.items[0]?.createdSeq : session.groups.find(group => group.id === entry.groupId)?.order_seq;
    animateEntry = animateEntry && (entry.kind === 'item' && entry.item.id.startsWith('pending:') || Boolean(created && sync?.snapshotSeq && compareRevision(created, sync.snapshotSeq) > 0));
    if (entry.kind === 'tool_group') {
      return (
        <div key={entry.id} data-timeline-key={entry.id} className={animateEntry ? 'motion-safe:animate-[timeline-enter_120ms_ease-out]' : undefined}>
          <ToolGroupRow items={entry.items} />
        </div>
      );
    }
    if (entry.kind === 'run_summary') {
      return (
        <RunSummaryBlock
          key={entry.id}
          entry={entry}
          usage={session.tokenUsageByRun[entry.runId]}
          animateEntry={animateEntry}
          renderEntry={(child) => renderEntry(child, false)}
          onLoad={(preserve) => loadGroup(entry.groupId, preserve)}
        />
      );
    }
    if (entry.item.id === runningTurn?.id) {
      return (
        <React.Fragment key={entry.item.id}>
          {renderItem(entry.item, animateEntry)}
          <RunningRunHeader clock={session.runClock ?? { elapsedMs: 0, runningSince: null }} logoSrc={runningLogoSrc} usage={session.tokenUsageByRun[activeRunId ?? '']} waiting={status === 'waiting'} />
        </React.Fragment>
      );
    }
    return renderItem(entry.item, animateEntry);
  };

  return (
    <div className="relative min-h-0 flex-1 bg-panel">
      <div
        ref={containerRef}
        data-testid="timeline-scroll"
        onScroll={handleScroll}
        style={{ overflowAnchor: 'none' }}
        className="scrollbar-none h-full space-y-2 overflow-y-auto p-3"
      >
        {session.loadStatus === 'error' && <div role="alert" className="flex items-center gap-2 py-2 text-sm text-danger">
          <AlertTriangle className="h-4 w-4 shrink-0" aria-hidden="true" />
          <span className="min-w-0 flex-1 break-words">{session.error?.message}</span>
          <button type="button" className="ui-interactive rounded-control p-2" aria-label="重试" onClick={() => {
            if (threadId) void useSessionStore.getState().retry(threadId);
            else if (activeProjectId) void useThreadStore.getState().loadThreads(activeProjectId);
          }}><RotateCcw className="h-4 w-4" /></button>
        </div>}
        {initialLoading ? <TimelineSkeleton /> : showEmptyWordmark ? (
          <EmptyTimeline actionsVisible={starterAvailable} onSelect={onStarterSelect} />
        ) : (
          <div className="space-y-2" aria-busy={session.loadStatus === 'refreshing'} {...(session.loadStatus !== 'ready' ? { inert: '' } : {})}>
            {sync?.hasMore && <button type="button" className="ui-interactive flex w-full items-center justify-center gap-1 rounded-control py-2 text-xs text-text-600" disabled={sync.historyPage.status === 'loading'} onClick={loadOlder}>
              {sync.historyPage.status === 'loading' ? <Loader2 className="h-4 w-4 animate-spin motion-reduce:animate-none" /> : <ArrowUp className="h-3.5 w-3.5" />}加载更多
            </button>}
            {sync?.historyPage.error && <p role="alert" className="text-xs text-danger">{sync.historyPage.error.message}</p>}
            {earlierEntries.map((entry) => renderEntry(entry))}
            <div
              ref={latestTurnRef}
              data-testid="latest-turn"
              className={`space-y-2 ${anchoredTurnId === latestTurnId && latestTurnId ? 'min-h-full' : ''}`}
            >
              {latestEntries.map((entry) => renderEntry(entry))}
              {['creating', 'running', 'recovering', 'canceling'].includes(status) && visibleProgress && (
                <LiveProgressRow progress={visibleProgress} />
              )}
              {commitActive && <GitCommitProgress phase={commitSession?.phase ?? null} />}
            </div>
          </div>
        )}
      </div>
      {showReturn && (
        <button
          type="button"
          onClick={() => scrollToLatest(true)}
          className="absolute bottom-3 left-1/2 inline-flex -translate-x-1/2 items-center gap-1 rounded-full border border-border bg-surface px-3 py-1.5 text-xs text-text-900 shadow-sm"
        >
          <ArrowDown className="h-3.5 w-3.5" strokeWidth={1.75} />
          回到最新
        </button>
      )}
    </div>
  );
};

function RunSummaryBlock({
  entry,
  usage,
  animateEntry,
  renderEntry,
  onLoad,
}: {
  entry: Extract<DisplayEntry, { kind: 'run_summary' }>;
  usage?: RunTokenUsage;
  animateEntry: boolean;
  renderEntry: (entry: DisplayEntry) => React.ReactNode;
  onLoad: (preserveAnchor: boolean) => void;
}) {
  const [expanded, setExpanded] = useState(false);
  const threadId = useActiveThreadId();
  const page = useSessionStore(state => threadId ? state.sessions[threadId]?.groupPages[entry.groupId] : undefined);
  const more = !page || page.hasMore;
  const load = () => onLoad(true);
  const terminal = entry.terminalItem;
  const duration = formatDuration(entry.durationMs);
  const tokens = formatRunTokenUsage(usage);
  const label = `${runSummaryLabel[entry.status]}，耗时 ${duration}${tokens !== null ? `，耗费 ${tokens} Tokens` : ''}`;

  return (
    <div data-timeline-key={entry.id} className={animateEntry ? 'motion-safe:animate-[timeline-enter_120ms_ease-out]' : undefined}>
      <button
        type="button"
        aria-expanded={expanded}
        onClick={() => { if (!expanded && more && page?.mode !== 'page') onLoad(false); setExpanded(value => !value); }}
        className={runHeaderClassName}
      >
        <RunStatusIcon status={entry.status} />
        <span className="min-w-0 flex-1 truncate leading-5">{label}</span>
        <ChevronRight
          className={`h-3.5 w-3.5 shrink-0 text-text-400 transition-transform duration-300 ease-out ${expanded ? 'rotate-90' : ''}`}
          strokeWidth={1.75}
        />
      </button>
      <TimelineDisclosure open={expanded}>
        {expanded && <div className="timeline-disclosure-rows mt-2 border-t border-border pt-2">
          {more && <button type="button" className="ui-interactive flex w-full items-center justify-center gap-1 rounded-control py-2 text-xs text-text-600" disabled={page?.status === 'loading'} onClick={load}>
            {page?.status === 'loading' ? <Loader2 className="h-4 w-4 animate-spin motion-reduce:animate-none" /> : <ArrowUp className="h-3.5 w-3.5" />}加载更多
          </button>}
          {page?.error && <p role="alert" className="text-xs text-danger">{page.error.message}</p>}
          {entry.processEntries.map(renderEntry)}
        </div>}
      </TimelineDisclosure>
      {terminal && <div className={`mt-1.5 pt-1.5${expanded ? '' : ' border-t border-border'}`}>
        {renderEntry({ kind: 'item', item: terminal })}
      </div>}
    </div>
  );
}


function TimelineSkeleton() {
  return <div aria-busy="true" aria-label="加载中" className="space-y-6 py-4 motion-safe:animate-pulse">
    <div className="ml-auto h-12 w-3/4 rounded-card bg-panel-muted" />
    <div className="space-y-2"><div className="h-3 w-4/5 rounded-control bg-panel-muted" /><div className="h-3 w-2/3 rounded-control bg-panel-muted" /></div>
    <div className="h-3 w-1/2 rounded-control bg-panel-muted" />
  </div>;
}

function TimelineDetail({ threadId, item }: { threadId: string; item: TimelineItem }) {
  const state = useSessionStore(value => value.sessions[threadId]?.detailStates[item.id]);
  return <div className="py-1">
    {state?.error && <p role="alert" className="text-xs text-danger">{state.error.message}</p>}
    <button type="button" className="ui-interactive inline-flex items-center gap-1 rounded-control px-2 py-1 text-xs text-text-600"
      disabled={state?.status === 'loading'} onClick={() => void useSessionStore.getState().loadDetail(threadId, item.id)}>
      {state?.status === 'loading' ? <Loader2 className="h-3.5 w-3.5 animate-spin motion-reduce:animate-none" /> : <ChevronRight className="h-3.5 w-3.5" />}展开全部
    </button>
  </div>;
}

function GroupPageButton({ threadId, groupId, onLoad }: { threadId: string | null; groupId: string; onLoad: () => void }) {
  const page = useSessionStore(state => threadId ? state.sessions[threadId]?.groupPages[groupId] : undefined);
  if (page && !page.hasMore) return null;
  return <div>
    <button type="button" className="ui-interactive flex w-full items-center justify-center gap-1 rounded-control py-2 text-xs text-text-600"
      disabled={page?.status === 'loading'} onClick={onLoad}>
      {page?.status === 'loading' ? <Loader2 className="h-4 w-4 animate-spin motion-reduce:animate-none" /> : <ArrowUp className="h-3.5 w-3.5" />}加载更多
    </button>
    {page?.error && <p role="alert" className="text-xs text-danger">{page.error.message}</p>}
  </div>;
}
