import React, { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { AlertTriangle, ArrowDown, CheckCircle2, ChevronRight, PauseCircle, StopCircle, XCircle } from 'lucide-react';
import { useDeckStore } from '../../stores/deckStore';
import { targetLabel } from './runtimeLabels';
import { useActiveSession } from './useActiveSession';
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
import { ComponentActivity, SkillActivity } from './SkillActivity';
import { useProjectStore } from '../../stores/projectStore';
import { useGitCommitStore } from '../../stores/gitCommitStore';

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
  canceled: '执行取消',
  failed: '执行错误',
  error: '系统异常',
} as const;

function fallbackProgress(status: ReturnType<typeof useActiveSession>['status']) {
  if (status === 'creating') return { stage: 'thinking' as const, text: '分析请求中' };
  if (status === 'running') return { stage: 'thinking' as const, text: '分析任务需求中' };
  if (status === 'canceling') return { stage: 'thinking' as const, text: '取消任务中' };
  return null;
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
  const { activeRunId, timelineItems, status, plan, progress } = session;
  const currentSlideId = useDeckStore((state) => state.currentSlideId);
  const activeProjectId = useProjectStore((state) => state.activeProjectId);
  const commitSession = useGitCommitStore((state) => (
    activeProjectId ? state.sessions[activeProjectId] : undefined
  ));
  const containerRef = useRef<HTMLDivElement>(null);
  const followingRef = useRef(true);
  const [showReturn, setShowReturn] = useState(false);
  const reducedMotion = typeof window !== 'undefined'
    && typeof window.matchMedia === 'function'
    && window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  const displayEntries = useMemo(
    () => groupTimelineItems(timelineItems, currentSlideId ?? undefined),
    [currentSlideId, timelineItems],
  );
  const displayedProgress = progress ?? fallbackProgress(status);
  const commitActive = commitSession?.status === 'creating' || commitSession?.status === 'running';
  const showEmptyWordmark = timelineItems.length === 0 && !plan && status === 'idle' && !commitActive;

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
    setShowReturn(false);
  }, [reducedMotion]);

  useLayoutEffect(() => {
    if (followingRef.current) scrollToLatest(true);
    else setShowReturn(true);
  }, [plan?.revision, scrollToLatest, timelineItems]);

  useEffect(() => {
    if (followingRef.current && progress) scrollToLatest(false);
  }, [progress, scrollToLatest]);

  const handleScroll = () => {
    const container = containerRef.current;
    if (!container) return;
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
                <div className="rounded-[10px] border border-border bg-panel-muted px-3 py-2">
                  <MarkdownMessage content={item.text} />
                  {item.deliveryStatus && (
                    <div className={`mt-1 text-[10px] ${
                      item.deliveryStatus === 'rejected' ? 'text-danger' : 'text-text-400'
                    }`}>
                      {item.deliveryStatus === 'sending'
                        ? '发送中'
                        : item.deliveryStatus === 'accepted'
                          ? '已接收'
                          : '未能加入当前任务'}
                    </div>
                  )}
                </div>
                <MessageMetaActions
                  text={item.text}
                  timestamp={item.timestamp}
                  label="复制用户消息"
                  scopeLabel={item.scope
                    ? targetLabel(item.scope.artifact as 'spec' | 'ppt', item.scope.level as 'slide' | 'deck')
                    : undefined}
                />
              </div>
            </div>
            {item.skills && item.skills.length > 0 && (
              <div className="mt-1">
                <SkillActivity skills={item.skills} />
              </div>
            )}
            {item.components && item.components.length > 0 && (
              <div className="mt-1">
                <ComponentActivity components={item.components} />
              </div>
            )}
          </>
        )}
        {item.type === 'run_lifecycle' && <RunLifecycleRow item={item} />}
        {item.type === 'reasoning' && <ReasoningRow item={item} />}
        {item.type === 'milestone' && <MilestoneRow item={item} />}
        {item.type === 'tool' && <ToolActivityRow item={item} />}
        {item.type === 'question' && <QuestionPanel item={item} />}
        {item.type === 'plan_approval' && <PlanApproval item={item} />}
        {item.type === 'command_permission' && <CommandPermissionCard item={item} />}
        {item.type === 'final' && <FinalMessage item={item} />}
        {item.type === 'terminal_notice' && <TerminalNotice item={item} />}
        {item.type === 'git_commit' && <GitCommitEvent item={item} />}
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
    return renderItem(entry.item, animateEntry);
  };

  return (
    <div className="relative min-h-0 flex-1 bg-panel">
      <div
        ref={containerRef}
        onScroll={handleScroll}
        className="h-full space-y-2 overflow-y-auto p-3"
      >
        {showEmptyWordmark ? (
          <div className="flex h-full items-center justify-center overflow-hidden">
            <EmptyTimelineTitle />
          </div>
        ) : (
          <>
            {displayEntries.map((entry) => renderEntry(entry))}
            {status === 'paused' && activeRunId && <PausedRunCard runId={activeRunId} />}
            {status !== 'waiting' && displayedProgress && (
              <LiveProgressRow progress={displayedProgress} />
            )}
            {commitActive && commitSession?.phase && <GitCommitProgress phase={commitSession.phase} />}
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
    ? '执行中断，任务已暂停'
    : `${runSummaryLabel[entry.status]}，耗时 ${duration}`;
  const finalText = terminal.type === 'final' ? terminal.text : terminal.message;

  return (
    <div className={animateEntry ? 'motion-safe:animate-[timeline-enter_120ms_ease-out]' : undefined}>
      <button
        type="button"
        aria-expanded={expanded}
        onClick={() => setExpanded((value) => !value)}
        className="flex min-h-8 w-full items-center gap-2 px-1.5 py-1 text-left text-[13px] text-text-600"
      >
        <RunStatusIcon status={superseded ? 'paused' : entry.status} />
        <span className="min-w-0 flex-1 truncate font-semibold">{label}</span>
        <ChevronRight
          className={`h-3.5 w-3.5 shrink-0 text-text-400 transition-transform duration-300 ease-out ${expanded ? 'rotate-90' : ''}`}
          strokeWidth={1.75}
        />
      </button>
      <TimelineDisclosure open={expanded && entry.processEntries.length > 0}>
        {expanded && entry.processEntries.length > 0 && <div className="timeline-disclosure-rows mt-1.5 border-t border-border pt-1.5">
          {entry.processEntries.map(renderEntry)}
        </div>}
      </TimelineDisclosure>
      <div className="mt-1.5 border-t border-border pt-1.5">
        {terminal.type === 'final'
          ? <FinalMessage item={terminal} />
          : (
            <article className="pb-4 pt-2 text-sm leading-[1.65] text-text-900">
              <MarkdownMessage content={finalText} />
            </article>
          )}
      </div>
    </div>
  );
}
