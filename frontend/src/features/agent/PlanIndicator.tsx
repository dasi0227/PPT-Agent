import React from 'react';
import { Check, LoaderCircle, ListChecks, X } from 'lucide-react';
import type { PlanState, PlanStep, PlanStepStatus } from '../../api/types';
import { cn } from '../../lib/utils';
import { IconButton } from '../../components/ui/primitives';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from '../../components/ui/dropdown-menu';

function StepNode({ status, animateCompletion }: { status: PlanStepStatus; animateCompletion: boolean }) {
  const nodeClasses = 'relative z-10 grid h-5 w-5 shrink-0 place-items-center rounded-full border-2 bg-surface transition-colors duration-200';
  switch (status) {
    case 'completed':
      return (
        <span
          role="img"
          aria-label="已完成"
          data-plan-step-status="completed"
          className={cn(nodeClasses, 'border-success/45 bg-success-soft text-success', animateCompletion && 'plan-step-node-completed')}
        >
          <Check className="h-3 w-3" strokeWidth={2.4} />
        </span>
      );
    case 'in_progress':
      return (
        <span role="img" aria-label="正在执行" data-plan-step-status="in_progress" className={cn(nodeClasses, 'border-accent/25 text-accent')}>
          <LoaderCircle className="h-3 w-3 animate-spin motion-reduce:animate-none" strokeWidth={2.2} />
        </span>
      );
    case 'failed':
      return (
        <span role="img" aria-label="执行失败" data-plan-step-status="failed" className={cn(nodeClasses, 'border-danger bg-danger-soft text-danger')}>
          <X className="h-3 w-3" strokeWidth={2.2} />
        </span>
      );
    default:
      return <span role="img" aria-label="等待执行" data-plan-step-status="pending" className={cn(nodeClasses, 'border-border-strong')} />;
  }
}

function connectorReached(step: PlanStep, nextStep: PlanStep | undefined): boolean {
  return step.status === 'completed' && nextStep !== undefined && nextStep.status !== 'pending';
}

function PlanStepRow({ step, nextStep }: { step: PlanStep; nextStep?: PlanStep }) {
  const previousStatus = React.useRef(step.status);
  const animateCompletion = previousStatus.current !== 'completed' && step.status === 'completed';
  const reached = connectorReached(step, nextStep);

  React.useEffect(() => {
    previousStatus.current = step.status;
  }, [step.status]);

  return (
    <li
      className={cn(
        'relative grid min-h-10 grid-cols-[20px_minmax(0,1fr)] items-center gap-2.5 rounded-lg px-2 py-2 text-sm',
        step.status === 'in_progress' && 'bg-accent-soft/80',
      )}
    >
      <StepNode status={step.status} animateCompletion={animateCompletion} />
      {nextStep && (
        <span
          data-plan-step-connector="true"
          data-reached={reached || undefined}
          aria-hidden="true"
          className="absolute -bottom-2.5 left-[17px] top-[30px] w-0.5 overflow-hidden rounded-full bg-border"
        >
          <span className={cn(
            'plan-step-connector-fill block h-full w-full origin-top scale-y-0 rounded-full bg-success/70',
            reached && 'scale-y-100',
          )} />
        </span>
      )}
      <PlanText className="max-w-[238px] leading-5 text-text-900">
        {step.title}
      </PlanText>
    </li>
  );
}

function RollingCharacter({ character }: { character: string }) {
  const previous = React.useRef(character);
  const [transition, setTransition] = React.useState<{ from: string; to: string } | null>(null);
  const [active, setActive] = React.useState(false);

  React.useEffect(() => {
    if (character === previous.current) return undefined;
    const from = previous.current;
    previous.current = character;
    setTransition({ from, to: character });
    setActive(false);
    const frame = requestAnimationFrame(() => requestAnimationFrame(() => setActive(true)));
    const done = window.setTimeout(() => setTransition(null), 320);
    return () => {
      cancelAnimationFrame(frame);
      window.clearTimeout(done);
    };
  }, [character]);

  return (
    <span className="plan-count-character">
      {transition ? (
        <span className={cn('plan-count-character-track', active && 'is-active')}>
          <span>{transition.from}</span>
          <span>{transition.to}</span>
        </span>
      ) : character}
    </span>
  );
}

function RollingCount({ completed, total }: { completed: number; total: number }) {
  const value = `${completed}/${total}`;
  return (
    <span className="inline-flex tabular-nums" aria-label={`${completed} / ${total}`}>
      <span aria-hidden="true" className="inline-flex">
        {value.split('').map((character, index) => (
          <RollingCharacter key={index} character={character} />
        ))}
      </span>
    </span>
  );
}

interface PlanIndicatorProps {
  plan?: PlanState | null;
  running: boolean;
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

export const PlanIndicator: React.FC<PlanIndicatorProps> = ({ plan, running }) => {
  const dismissedByPointerRef = React.useRef(false);
  const hasPlan = Boolean(plan && plan.steps.length > 0);
  const total = plan?.steps.length ?? 0;
  const completed = plan?.steps.filter((step) => step.status === 'completed').length ?? 0;
  const inFlight = running && completed < total;

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <IconButton
          label={hasPlan ? `查看计划进度 ${completed} / ${total}` : '暂无计划'}
          aria-haspopup="menu"
        >
          <ListChecks
            className={cn('h-4 w-4', inFlight && 'animate-pulse motion-reduce:animate-none')}
            strokeWidth={1.75}
          />
        </IconButton>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        side="bottom"
        align="end"
        className="w-[320px] overflow-visible p-2"
        onPointerDownOutside={() => {
          dismissedByPointerRef.current = true;
        }}
        onEscapeKeyDown={() => {
          dismissedByPointerRef.current = false;
        }}
        onCloseAutoFocus={(event) => {
          if (dismissedByPointerRef.current) event.preventDefault();
          dismissedByPointerRef.current = false;
        }}
      >
        {hasPlan ? (
          <>
            <div className="mb-1.5 flex items-center gap-2 px-1">
              <PlanText className="max-w-[248px] text-sm font-semibold text-text-900">
                {plan?.title || '执行计划'}
              </PlanText>
              <span className="shrink-0 text-xs text-text-400">
                <RollingCount completed={completed} total={total} />
              </span>
            </div>
            <ol className="max-h-[280px] overflow-y-auto pr-1">
              {plan?.steps.map((step, index) => (
                <PlanStepRow key={step.id} step={step} nextStep={plan.steps[index + 1]} />
              ))}
            </ol>
          </>
        ) : (
          <div className="flex items-center justify-center py-4 text-sm text-text-400">暂无计划</div>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
};
