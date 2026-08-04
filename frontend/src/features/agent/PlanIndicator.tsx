import React from 'react';
import { CheckCircle2, Circle, ListChecks, Loader2, XCircle } from 'lucide-react';
import type { PlanState, PlanStepStatus } from '../../api/types';
import { cn } from '../../lib/utils';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from '../../components/ui/dropdown-menu';

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

const stepTitleClass = (status: PlanStepStatus) =>
  status === 'failed'
    ? 'text-danger'
    : status === 'completed'
      ? 'text-text-600'
      : 'text-text-900';

interface PlanIndicatorProps {
  plan: PlanState;
  running: boolean;
}

export const PlanIndicator: React.FC<PlanIndicatorProps> = ({ plan, running }) => {
  const total = plan.steps.length;
  const completed = plan.steps.filter((step) => step.status === 'completed').length;
  const inFlight = running && completed < total;

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          aria-label={`计划 ${completed} / ${total}`}
          className="inline-flex h-7 min-w-0 shrink items-center gap-1 rounded-md border border-border bg-transparent px-1.5 text-[11px] font-medium text-text-600 transition-colors hover:bg-panel-muted hover:text-text-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
        >
          {inFlight ? (
            <span className="relative flex h-2 w-2 shrink-0" aria-hidden>
              <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-accent opacity-60 motion-reduce:animate-none" />
              <span className="relative inline-flex h-2 w-2 rounded-full bg-accent" />
            </span>
          ) : (
            <ListChecks className="h-3.5 w-3.5 shrink-0" strokeWidth={1.75} />
          )}
          <span className="shrink-0 whitespace-nowrap">计划</span>
          <span className="shrink-0 tabular-nums">{completed} / {total}</span>
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent side="top" align="start" className="max-h-[320px] w-[288px] overflow-y-auto p-2">
        <div className="mb-1.5 flex items-center gap-2 px-1">
          <span className="min-w-0 flex-1 truncate text-sm font-semibold text-text-900">
            {plan.title || '执行计划'}
          </span>
          <span className="shrink-0 text-xs tabular-nums text-text-400">{completed}/{total}</span>
        </div>
        <div className="space-y-0.5">
          {plan.steps.map((step) => (
            <div
              key={step.id}
              className={cn(
                'flex items-start gap-2 rounded-lg px-2 py-1.5 text-sm',
                step.status === 'in_progress' && 'bg-accent-soft',
              )}
            >
              <StepIcon status={step.status} />
              <div className="min-w-0 flex-1">
                <div className={cn('leading-5', stepTitleClass(step.status))}>
                  {step.title}
                </div>
                {step.detail && (
                  <div className="mt-0.5 text-xs leading-4 text-text-400">{step.detail}</div>
                )}
              </div>
            </div>
          ))}
        </div>
      </DropdownMenuContent>
    </DropdownMenu>
  );
};
