import { AgentCommandRow } from './AgentCommandRow';
import { TextCommandActivity } from './TextCommandActivity';
import { RollbackButton } from './ProjectHistoryControls';
import React, { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { AlertTriangle, ArrowDown, CheckCircle2, ChevronRight, Code2, FileImage, PauseCircle, StopCircle, XCircle } from 'lucide-react';
import { useDeckStore } from '../../stores/deckStore';
import { targetLabel } from './runtimeLabels';
import { useActiveSession, useActiveThreadId } from './useActiveSession';
import { FinalMessage } from './FinalMessage';
import { LiveProgressRow } from './LiveProgressRow';
import { MarkdownMessage } from './MarkdownMessage';
import { QuestionPanel } from './QuestionPanel';
import { ReasoningRow, MilestoneRow, RunLifecycleRow, ToolActivityRow, ToolGroupRow } from './ActivityRows';
import { TerminalNotice } from './TerminalNotice';
import { PlanApproval } from './PlanApproval';
import { MessageMetaActions } from './MessageMetaActions';
import type { TimelineItem } from './eventReducer';
import { DisplayEntry, groupTimelineItems } from './timelineGrouping';
import { PausedRunCard } from './PausedRunCard';
import { TimelineDisclosure } from './TimelineDisclosure';
import { CommandPermissionCard } from './CommandPermissionCard';
import { GitCommitEvent, GitCommitProgress } from './GitCommitActivity';
import { useProjectStore } from '../../stores/projectStore';
import { useGitCommitStore } from '../../stores/gitCommitStore';
import { BriefingActivity } from './BriefingActivity';
import { ContextCompactionActivity } from './ContextCompactionActivity';
import { ScopeExpansionCard } from './ScopeExpansionCard';
import { useCommandHistoryRecovery } from './useCommandHistoryRecovery';
import { attachmentsApi } from '../../api/attachments';
import { ImagePreview } from '../../components/ui/ImagePreview';
import { runElapsed, type RunClock } from './runClock';

const messageReferenceClassName = 'inline-flex h-7 shrink-0 items-center gap-1 rounded-md border border-border bg-surface px-2 text-[11px] font-semibold text-text-900 ui-interactive';

function EmptyTimelineTitle() {
  return <p className="text-center text-2xl font-bold italic tracking-tight text-text-400">Dasi PPT Agent</p>;
}

function formatDuration(durationMs?: number): string {
  if (durationMs === undefined || !Number.isFinite(durationMs) || durationMs < 0) return '--';
  const totalSeconds = Math.max(0, Math.round(durationMs / 1000));
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  if (minutes <= 0) return `${seconds}s`;
  return `${minutes}m ${seconds}s`;
}

const runSummaryLabel = {
  completed: '执行完成',
  canceled: '执行中断',
  failed: '执行失败',
  error: '系统异常',
} as const;

function RunningRunHeader({ clock }: { clock: RunClock }) {
  const [now, setNow] = useState(Date.now);
  useEffect(() => {
    setNow(Date.now());
    if (clock.runningSince === null) return;
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, [clock.runningSince]);
  return (
    <div className="pb-1.5">
      <div className="flex min-h-8 items-center py-1 text-[13px] text-text-600">
        <span>正在执行，已进行 {formatDuration(runElapsed(clock, now))}</span>
      </div>
      <div className="mt-1.5 border-t border-border" />
    </div>
  );
}

function RunStatusIcon({ status }: { status: 'completed' | 'failed' | 'error' | 'canceled' | 'paused' }) {
  if (status === 'completed') {
    return <CheckCircle2 className="h-4 w-4 shrink-0 text-success" strokeWidth={1.75} />;
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

export const Timeline: React.FC = () => {
  const session = useActiveSession();
  const threadId = useActiveThreadId();
  const { activeRunId, timelineItems, status, plan, progress } = session;
  useCommandHistoryRecovery(threadId, timelineItems);
  const currentSlideId = useDeckStore((state) => state.currentSlideId);
  const activeProjectId = useProjectStore((state) => state.activeProjectId);
  const commitSession = useGitCommitStore((state) => (
    activeProjectId ? state.sessions[activeProjectId] : undefined
  ));
  const containerRef = useRef<HTMLDivElement>(null);
  const latestTurnRef = useRef<HTMLDivElement>(null);
  const followingRef = useRef(true);
  const anchorTopRef = useRef<number | null>(null);
  const alignedTurnRef = useRef<string | null>(null);
  const previousTurnRef = useRef<{ threadId: string | null; id: string | null } | null>(null);
  const [showReturn, setShowReturn] = useState(false);
  const [anchoredTurnId, setAnchoredTurnId] = useState<string | null>(null);
	const [openDOMReference, setOpenDOMReference] = useState<{ key:string; marker:number; comment:string; status:string } | null>(null);
  const reducedMotion = typeof window !== 'undefined'
    && typeof window.matchMedia === 'function'
    && window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  const displayEntries = useMemo(
    () => groupTimelineItems(timelineItems, currentSlideId ?? undefined),
    [currentSlideId, timelineItems],
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
  const earlierEntries = latestTurnIndex < 0 ? [] : displayEntries.slice(0, latestTurnIndex);
  const latestEntries = latestTurnIndex < 0 ? displayEntries : displayEntries.slice(latestTurnIndex);
  const commitActive = commitSession?.sourceThreadId===threadId && (commitSession?.status === 'creating' || commitSession?.status === 'running');
  const showEmptyWordmark = timelineItems.length === 0 && !plan && status === 'idle' && !commitActive;
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

  useLayoutEffect(() => {
    const previous = previousTurnRef.current;
    previousTurnRef.current = { threadId, id: latestTurnId };
    if (!previous || previous.threadId !== threadId) {
      alignedTurnRef.current = null;
      anchorTopRef.current = null;
      followingRef.current = true;
      setAnchoredTurnId(status === 'creating' && latestTurnId?.startsWith('user_') ? latestTurnId : null);
      return;
    }
    if (latestTurnId && latestTurnId !== previous.id) {
      alignedTurnRef.current = null;
      followingRef.current = false;
      setAnchoredTurnId(latestTurnId);
    }
  }, [latestTurnId, status, threadId]);

  useLayoutEffect(() => {
    const container = containerRef.current;
    if (anchoredTurnId && anchoredTurnId === latestTurnId && latestTurnRef.current
      && alignedTurnRef.current !== `${threadId}:${anchoredTurnId}` && container) {
      const targetTop = latestTurnRef.current.getBoundingClientRect().top
        - container.getBoundingClientRect().top + container.scrollTop;
      container.scrollTop = Math.max(0, targetTop - 12);
      anchorTopRef.current = container.scrollTop;
      alignedTurnRef.current = `${threadId}:${anchoredTurnId}`;
      followingRef.current = false;
      setShowReturn(false);
      return;
    }
    if (followingRef.current) scrollToLatest(true);
    else if (container) setShowReturn(container.scrollHeight - container.scrollTop - container.clientHeight > 80);
  }, [anchoredTurnId, latestTurnId, plan, scrollToLatest, threadId, timelineItems]);

  useEffect(() => {
    if (followingRef.current && progress) scrollToLatest(false);
  }, [progress, scrollToLatest]);

  const handleScroll = () => {
    const container = containerRef.current;
    if (!container) return;
    if (anchorTopRef.current !== null && Math.abs(container.scrollTop - anchorTopRef.current) <= 1) return;
    anchorTopRef.current = null;
    const nearBottom = container.scrollHeight - container.scrollTop - container.clientHeight <= 80;
    followingRef.current = nearBottom;
    setShowReturn(!nearBottom);
  };

  const renderItem = (item: TimelineItem, animateEntry = true) => {
    return (
      <div key={item.id} className={animateEntry ? 'motion-safe:animate-[timeline-enter_120ms_ease-out]' : undefined}>
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
						return <button key={key} type="button" onClick={() => setOpenDOMReference(openDOMReference?.key === key ? null : { key, marker:selection.marker_no, comment:selection.comment, status:selection.status })} className={messageReferenceClassName}><Code2 className="h-3.5 w-3.5 text-accent" />标记 {selection.marker_no}</button>;
					  })}
					</div>
				  )}
				  {openDOMReference?.key.startsWith(`${item.id}:`) && <div className="mb-2 rounded-md border border-border bg-surface p-2 text-xs text-text-600"><div className="whitespace-pre-wrap">{openDOMReference.comment || '未填写注释'}</div>{openDOMReference.status !== 'active' && <div className="mt-1 text-warning">{openDOMReference.status === 'page_deleted' ? '页面已删除' : '内容已删除'}</div>}</div>}
                  <MarkdownMessage content={item.text} />
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
                ><RollbackButton runId={item.runId} steering={Boolean(item.deliveryStatus)} /></MessageMetaActions>
              </div>
            </div>

          </>
        )}
        {item.type === 'run_lifecycle' && <RunLifecycleRow item={item} />}
        {item.type === 'reasoning' && <ReasoningRow item={item} />}
        {item.type === 'milestone' && <MilestoneRow item={item} />}
        {item.type === 'tool' && <ToolActivityRow item={item} />}
        {item.type === 'question' && <QuestionPanel item={item} />}
        {item.type === 'plan_approval' && <PlanApproval item={item} />}
        {item.type === 'command_permission' && <CommandPermissionCard item={item} />}
        {item.type === 'scope_expansion' && <ScopeExpansionCard item={item} />}
        {item.type === 'final' && <FinalMessage item={item} />}
        {item.type === 'terminal_notice' && <TerminalNotice item={item} />}
        {item.type === 'git_commit' && (item.commandSource === 'automatic' ? <AgentCommandRow item={item} /> : !(item.status === 'loading' && commitActive && commitSession?.operationId === item.operationId) && <GitCommitEvent item={item} />)}
        {item.type === 'command' && (item.commandSource === 'automatic' && (item.kind === 'rename' || item.kind === 'compact') ? <AgentCommandRow item={item} /> : <TextCommandActivity item={item} />)}
        {item.type === 'briefing' && <BriefingActivity item={item} />}
        {item.type === 'context_compaction' && (item.trigger === 'auto' ? <AgentCommandRow item={item} /> : <ContextCompactionActivity item={item} />)}
      </div>
    );
  };

  const renderEntry = (entry: DisplayEntry, animateEntry = true): React.ReactNode => {
    if (entry.kind === 'tool_group') {
      return (
        <div key={entry.id} className={animateEntry ? 'motion-safe:animate-[timeline-enter_120ms_ease-out]' : undefined}>
          <ToolGroupRow items={entry.items} />
        </div>
      );
    }
    if (entry.kind === 'run_summary') {
      return (
        <RunSummaryBlock
          key={entry.id}
          entry={entry}
          animateEntry={animateEntry}
          renderEntry={(child) => renderEntry(child, false)}
        />
      );
    }
    if (entry.item.id === runningTurn?.id) {
      return (
        <React.Fragment key={entry.item.id}>
          {renderItem(entry.item, animateEntry)}
          <RunningRunHeader clock={session.runClock ?? { elapsedMs: 0, runningSince: null }} />
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
        className="scrollbar-none h-full space-y-2 overflow-y-auto p-3"
      >
        {showEmptyWordmark ? (
          <div className="flex h-full items-center justify-center overflow-hidden">
            <EmptyTimelineTitle />
          </div>
        ) : (
          <>
            {earlierEntries.map((entry) => renderEntry(entry))}
            <div
              ref={latestTurnRef}
              data-testid="latest-turn"
              className={`space-y-2 ${anchoredTurnId === latestTurnId && latestTurnId ? 'min-h-full' : ''}`}
            >
              {latestEntries.map((entry) => renderEntry(entry))}
              {status === 'paused' && activeRunId && <PausedRunCard runId={activeRunId} />}
              {status !== 'waiting' && visibleProgress && (
                <LiveProgressRow progress={visibleProgress} />
              )}
              {commitActive && <GitCommitProgress phase={commitSession?.phase ?? null} />}
            </div>
          </>
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
  animateEntry,
  renderEntry,
}: {
  entry: Extract<DisplayEntry, { kind: 'run_summary' }>;
  animateEntry: boolean;
  renderEntry: (entry: DisplayEntry) => React.ReactNode;
}) {
  const [expanded, setExpanded] = useState(false);
  const terminal = entry.terminalItem;
  const duration = formatDuration(terminal.durationMs);
  const superseded = terminal.type === 'terminal_notice' && terminal.reason === 'superseded';
  const label = superseded
    ? '任务执行异常'
    : `${runSummaryLabel[entry.status]}，耗时 ${duration}`;

  return (
    <div className={animateEntry ? 'motion-safe:animate-[timeline-enter_120ms_ease-out]' : undefined}>
      <button
        type="button"
        aria-expanded={expanded}
        onClick={() => setExpanded((value) => !value)}
        className="flex min-h-8 w-full items-center gap-2 px-1.5 py-1 text-left text-[13px] text-text-600"
      >
        <RunStatusIcon status={superseded ? 'error' : entry.status} />
        <span className="min-w-0 flex-1 truncate">{label}</span>
        <ChevronRight
          className={`h-3.5 w-3.5 shrink-0 text-text-400 transition-transform duration-300 ease-out ${expanded ? 'rotate-90' : ''}`}
          strokeWidth={1.75}
        />
      </button>
      <TimelineDisclosure open={expanded && entry.processEntries.length > 0}>
        {expanded && entry.processEntries.length > 0 && <div className="timeline-disclosure-rows mt-2 border-t border-border pt-2">
          {entry.processEntries.map(renderEntry)}
        </div>}
      </TimelineDisclosure>
      <div className="mt-1.5 border-t border-border pt-1.5">
        {terminal.type === 'final'
          ? <FinalMessage item={terminal} />
          : (
            <TerminalNotice item={terminal} />
          )}
      </div>
    </div>
  );
}
