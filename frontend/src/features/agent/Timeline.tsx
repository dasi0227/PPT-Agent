import React, { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { ArrowDown } from 'lucide-react';
import { useDeckStore } from '../../stores/deckStore';
import { useProjectStore } from '../../stores/projectStore';
import { targetLabel } from './runtimeLabels';
import { useActiveSession } from './useActiveSession';
import { FinalMessage } from './FinalMessage';
import { LiveProgressRow } from './LiveProgressRow';
import { MarkdownMessage } from './MarkdownMessage';
import { PlanPanel } from './PlanPanel';
import { QuestionPanel } from './QuestionPanel';
import { ReasoningRow, MilestoneRow, ToolActivityRow, ToolGroupRow } from './ActivityRows';
import { TerminalNotice } from './TerminalNotice';
import type { TimelineItem } from './eventReducer';
import { groupTimelineItems } from './timelineGrouping';

function EmptyTimelineTitle() {
  return <p className="text-center text-2xl font-bold italic tracking-tight text-text-400">Dasi PPT Agent</p>;
}

export const Timeline: React.FC = () => {
  const session = useActiveSession();
  const { timelineItems, status, plan, progress } = session;
  const activeProjectId = useProjectStore((state) => state.activeProjectId);
  const slides = useProjectStore((state) => activeProjectId ? state.slidesByProjectId[activeProjectId] ?? [] : []);
  const currentPage = useDeckStore((state) => state.currentPage);
  const currentSlideId = slides[currentPage]?.id;
  const containerRef = useRef<HTMLDivElement>(null);
  const followingRef = useRef(true);
  const [showReturn, setShowReturn] = useState(false);
  const reducedMotion = typeof window !== 'undefined'
    && typeof window.matchMedia === 'function'
    && window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  const displayEntries = useMemo(
    () => groupTimelineItems(timelineItems, currentSlideId),
    [currentSlideId, timelineItems],
  );
  const lastUserIndex = timelineItems.reduce(
    (latest, item, index) => item.type === 'user_turn' ? index : latest,
    -1,
  );
  const runActive = status === 'creating' || status === 'running' || status === 'waiting' || status === 'canceling';
  const showEmptyWordmark = timelineItems.length === 0 && !plan && status === 'idle';

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

  const renderItem = (item: TimelineItem, sourceIndex?: number) => {
    const planAfter = plan && sourceIndex === lastUserIndex;
    return (
      <React.Fragment key={item.id}>
        {item.type === 'user_turn' && (
          <div className="flex justify-end">
            <div className="max-w-[88%] rounded-[10px] border border-border bg-panel-muted px-3 py-2">
              {item.target && (
                <div className="mb-1 text-[10px] font-medium text-text-400">
                  {targetLabel(item.target.artifact as 'spec' | 'presentation', item.target.level as 'slide' | 'deck')}
                </div>
              )}
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
          </div>
        )}
        {item.type === 'reasoning' && <ReasoningRow item={item} />}
        {item.type === 'milestone' && <MilestoneRow item={item} />}
        {item.type === 'tool' && <ToolActivityRow item={item} />}
        {item.type === 'question' && <QuestionPanel item={item} />}
        {item.type === 'final' && <FinalMessage item={item} />}
        {item.type === 'terminal_notice' && <TerminalNotice item={item} />}
        {planAfter && <PlanPanel plan={plan} running={runActive} />}
      </React.Fragment>
    );
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
            {plan && lastUserIndex < 0 && <PlanPanel plan={plan} running={runActive} />}
            {displayEntries.map((entry) => {
              if (entry.kind === 'tool_group') {
                return <ToolGroupRow key={entry.id} items={entry.items} />;
              }
              const sourceIndex = timelineItems.findIndex((item) => item.id === entry.item.id);
              return renderItem(entry.item, sourceIndex);
            })}
            {status !== 'waiting' && (progress || status === 'creating') && (
              <LiveProgressRow progress={progress ?? {
                stage: 'thinking',
                text: '正在启动 Agent',
              }} />
            )}
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
