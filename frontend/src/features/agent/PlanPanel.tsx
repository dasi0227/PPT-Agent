import React, { useEffect, useState } from 'react';
import {
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  Circle,
  ListTodo,
  Loader2,
  XCircle,
} from 'lucide-react';
import type { PlanState, PlanStepStatus } from '../../api/types';
import { cn } from '../../lib/utils';

function StepIcon({ status }: { status: PlanStepStatus }) {
  const classes = 'h-4 w-4 shrink-0';
  switch (status) {
    case 'completed':
      return <CheckCircle2 className={cn(classes, 'text-success')} strokeWidth={1.75} />;
    case 'in_progress':
      return <Loader2 className={cn(classes, 'animate-spin text-accent motion-reduce:animate-none')} strokeWidth={1.75} />;
    case 'failed':
      return <XCircle className={cn(classes, 'text-danger')} strokeWidth={1.75} />;
    default:
      return <Circle className={cn(classes, 'text-text-400')} strokeWidth={1.75} />;
  }
}

export const PlanPanel: React.FC<{ plan: PlanState; running: boolean }> = ({ plan, running }) => {
  const allDone = plan.steps.length > 0 && plan.steps.every((step) => step.status === 'completed');
  const [expanded, setExpanded] = useState(running && !allDone);
  const completed = plan.steps.filter((step) => step.status === 'completed').length;

  useEffect(() => {
    setExpanded(running && !allDone);
  }, [allDone, running]);

  return (
    <section className="overflow-hidden rounded-[10px] border border-border bg-surface motion-safe:animate-[timeline-enter_120ms_ease-out]">
      <button
        type="button"
        aria-expanded={expanded}
        onClick={() => setExpanded((value) => !value)}
        className="flex h-9 w-full items-center gap-2 px-3 text-left text-sm font-medium text-text-900 transition-colors hover:bg-panel-muted"
      >
        {expanded
          ? <ChevronDown className="h-4 w-4 text-text-400" strokeWidth={1.75} />
          : <ChevronRight className="h-4 w-4 text-text-400" strokeWidth={1.75} />}
        <ListTodo className="h-4 w-4 text-text-600" strokeWidth={1.75} />
        <span className="min-w-0 flex-1 truncate">{plan.title || '执行计划'}</span>
        <span className="text-xs tabular-nums text-text-400">{completed}/{plan.steps.length}</span>
      </button>
      {expanded && (
        <div className="space-y-1 border-t border-border px-2 py-2">
          {plan.steps.map((step) => (
            <div
              key={step.id}
              className={cn(
                'flex items-start gap-2 rounded-lg px-2 py-1.5 text-sm',
                step.status === 'in_progress' && 'bg-accent-soft',
              )}
            >
              <StepIcon status={step.status} />
              <span className={cn(
                'min-w-0 leading-5',
                step.status === 'failed' ? 'text-danger' : 'text-text-900',
                step.status === 'completed' && 'text-text-600',
              )}>
                {step.title}
              </span>
            </div>
          ))}
        </div>
      )}
    </section>
  );
};
