import React from 'react';
import { Check, Circle, ListChecks, Loader2, XCircle } from 'lucide-react';
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
      return <Check className={cn(classes, 'text-success')} strokeWidth={1.75} />;
    case 'in_progress':
      return <Loader2 className={cn(classes, 'animate-spin text-accent motion-reduce:animate-none')} strokeWidth={1.75} />;
    case 'failed':
      return <XCircle className={cn(classes, 'text-danger')} strokeWidth={1.75} />;
    default:
      return <Circle className={cn(classes, 'text-text-400')} strokeWidth={1.75} />;
  }
}

interface PlanIndicatorProps {
  plan?: PlanState | null;
  running: boolean;
  selected?: boolean;
  disabled?: boolean;
  onSelectPlan?: () => void;
}

function PlanText({
  children,
  className,
  tooltipClassName,
}: {
  children: string;
  className?: string;
  tooltipClassName?: string;
}) {
  const textRef = React.useRef<HTMLSpanElement>(null);
  const timerRef = React.useRef<number | null>(null);
  const [tooltip, setTooltip] = React.useState<{ top: number; left: number; width: number } | null>(null);

  React.useEffect(() => () => {
    if (timerRef.current !== null) window.clearTimeout(timerRef.current);
  }, []);

  const hide = () => {
    if (timerRef.current !== null) {
      window.clearTimeout(timerRef.current);
      timerRef.current = null;
    }
    setTooltip(null);
  };

  const showLater = () => {
    hide();
    timerRef.current = window.setTimeout(() => {
      const el = textRef.current;
      if (!el || el.scrollWidth <= el.clientWidth) return;
      const rect = el.getBoundingClientRect();
      setTooltip({
        top: rect.bottom + 6,
        left: rect.left,
        width: Math.min(Math.max(rect.width, 220), 360),
      });
    }, 500);
  };

  return (
    <>
      <span
        ref={textRef}
        className={cn('block min-w-0 truncate', className)}
        onMouseEnter={showLater}
        onMouseLeave={hide}
        onFocus={showLater}
        onBlur={hide}
        title={children}
      >
        {children}
      </span>
      {tooltip && (
        <span
          role="tooltip"
          className={cn(
            'fixed z-50 rounded-md border border-border bg-surface px-2 py-1 text-xs leading-5 text-text-900 shadow-overlay',
            tooltipClassName,
          )}
          style={{ top: tooltip.top, left: tooltip.left, maxWidth: tooltip.width }}
        >
          {children}
        </span>
      )}
    </>
  );
}

const planButtonClass = (selected: boolean) => cn(
  'composer-plan-button inline-flex h-7 min-w-0 shrink items-center gap-0.5 rounded-md border px-1 text-[11px] font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent disabled:cursor-not-allowed disabled:opacity-45',
  selected
    ? 'border-accent/30 bg-accent-soft text-accent'
    : 'border-border bg-transparent text-text-600 hover:bg-panel-muted hover:text-text-900',
);

export const PlanIndicator: React.FC<PlanIndicatorProps> = ({
  plan,
  running,
  selected = false,
  disabled = false,
  onSelectPlan,
}) => {
  const hasPlan = Boolean(plan && plan.steps.length > 0);
  const total = plan?.steps.length ?? 0;
  const completed = plan?.steps.filter((step) => step.status === 'completed').length ?? 0;
  const inFlight = running && completed < total;

  if (!hasPlan) {
    const togglePlan = () => {
      onSelectPlan?.();
    };
    return (
      <button
        type="button"
        aria-label="计划"
        aria-pressed={selected}
        title="只写计划并回显，不修改项目内容"
        disabled={disabled}
        onClick={togglePlan}
        className={planButtonClass(selected)}
      >
        <ListChecks className="h-3.5 w-3.5" strokeWidth={1.75} />
        <span className="composer-plan-label">计划</span>
      </button>
    );
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          aria-label={`计划 ${completed} / ${total}`}
          disabled={disabled}
          className={planButtonClass(false)}
        >
          <ListChecks className={cn('h-3.5 w-3.5 shrink-0', inFlight && 'animate-pulse motion-reduce:animate-none')} strokeWidth={1.75} />
          <span className="composer-plan-label shrink-0 whitespace-nowrap">计划</span>
          <span className="composer-plan-count shrink-0 tabular-nums">{completed} / {total}</span>
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent side="top" align="start" className="w-[320px] overflow-visible p-2">
        <div className="mb-1.5 flex items-center gap-2 px-1">
          <PlanText className="max-w-[248px] text-sm font-semibold text-text-900">
            {plan?.title || '执行计划'}
          </PlanText>
          <span className="shrink-0 text-xs tabular-nums text-text-400">{completed}/{total}</span>
        </div>
        <div className="max-h-[280px] space-y-0.5 overflow-y-auto pr-1">
          {plan?.steps.map((step) => (
            <div
              key={step.id}
              className={cn(
                'flex min-h-8 items-center gap-2 rounded-lg px-2 py-1.5 text-sm',
                step.status === 'in_progress' && 'bg-accent-soft',
              )}
            >
              <StepIcon status={step.status} />
              <div className="min-w-0 flex-1">
                <PlanText className="max-w-[238px] leading-5 text-text-900">
                  {step.title}
                </PlanText>
                {step.detail && (
                  <PlanText className="mt-0.5 max-w-[238px] text-xs leading-4 text-text-400">
                    {step.detail}
                  </PlanText>
                )}
              </div>
            </div>
          ))}
        </div>
      </DropdownMenuContent>
    </DropdownMenu>
  );
};
