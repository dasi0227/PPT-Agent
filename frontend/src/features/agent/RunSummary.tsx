import React from 'react';
import { Loader2, Square } from 'lucide-react';
import { Button, Badge } from '../../components/ui/primitives';
import { useRunStore } from '../../stores/runStore';
import { useActiveSession, useActiveThreadId } from './useActiveSession';
import { runStatusLabels, targetLabel } from './runtimeLabels';

export const RunSummary: React.FC = () => {
  const threadId = useActiveThreadId();
  const session = useActiveSession();
  const cancelRun = useRunStore((state) => state.cancelRun);
  const active = session.status === 'creating' || session.status === 'running' || session.status === 'waiting' || session.status === 'canceling';
  const show = active || ['done', 'error', 'canceled'].includes(session.status);
  if (!show) return null;

  const completed = session.plan?.steps.filter((step) => step.status === 'completed').length;
  const total = session.plan?.steps.length;
  const credibleProgress = total && total > 0 ? `${completed} / ${total}` : null;
  const tone = session.status === 'error'
    ? 'danger'
    : session.status === 'waiting'
      ? 'accent'
      : session.status === 'done'
        ? 'success'
        : session.status === 'canceled'
          ? 'neutral'
          : 'accent';

  return (
    <section aria-label="运行摘要" className="border-b border-border bg-panel-muted/60 px-3 py-2">
      <div className="flex items-center gap-2">
        {active && <Loader2 className="h-3.5 w-3.5 animate-spin text-accent" strokeWidth={1.75} />}
        <span className="min-w-0 flex-1 truncate text-xs font-medium text-text-900">
          {targetLabel(session.target.artifact, session.target.level)}
        </span>
        <Badge tone={tone}>{runStatusLabels[session.status]}</Badge>
        {active && threadId && session.activeRunId && (
          <Button
            variant="ghost"
            className="h-7 px-2 text-xs text-danger hover:bg-danger-soft hover:text-danger"
            onClick={() => void cancelRun(threadId, session.activeRunId!)}
            disabled={session.status === 'canceling'}
          >
            <Square className="h-3 w-3" fill="currentColor" /> 停止
          </Button>
        )}
      </div>
      <div className="mt-1.5 flex min-w-0 items-center gap-2 text-[11px] text-text-600">
        {credibleProgress && (
          <span className="font-mono tabular-nums">{credibleProgress}</span>
        )}
        {session.streamStatus === 'reconnecting' && (
          <span className="ml-auto text-warning">正在恢复连接</span>
        )}
      </div>
    </section>
  );
};
