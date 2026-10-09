import React from 'react';
import { Check, LoaderCircle, ListChecks, X } from 'lucide-react';
import type { PlanState, PlanStep, PlanStepStatus } from '../../api/types';
import { cn } from '../../lib/utils';
import { IconButton } from '../../components/ui/primitives';
import { ToolbarProgressBadge } from './ToolbarProgressBadge';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from '../../components/ui/dropdown-menu';

function StepNode({ status, animateCompletion }: { status: PlanStepStatus; animateCompletion: boolean }) {
  const nodeClasses = 'relative z-10 grid h-5 w-5 shrink-0 place-items-center rounded-full bg-surface transition-colors duration-200';
  switch (status) {
    case 'completed':
      return (
        <span
          role="img"
          aria-label="已完成"
          data-plan-step-status="completed"
          className={cn(nodeClasses, 'text-success', animateCompletion && 'plan-step-node-completed')}
        >
          <Check className="h-[17px] w-[17px]" strokeWidth={1.75} />
        </span>
      );
    case 'processing':
      return (
        <span role="img" aria-label="正在执行" data-plan-step-status="processing" className={cn(nodeClasses, 'bg-hover text-selected-foreground')}>
          <LoaderCircle className="h-[17px] w-[17px] animate-spin motion-reduce:animate-none" strokeWidth={1.75} />
        </span>
      );
    case 'failed':
      return (
        <span role="img" aria-label="执行失败" data-plan-step-status="failed" className={cn(nodeClasses, 'border-2 border-danger bg-danger-soft text-danger')}>
          <X className="h-3 w-3" strokeWidth={2.2} />
        </span>
      );
    default:
      return <span role="img" aria-label="等待执行" data-plan-step-status="pending" className={cn(nodeClasses, 'border-[1.25px] border-border-strong')} />;
  }
}

function PlanStepRow({ step, nextStep }: { step: PlanStep; nextStep?: PlanStep }) {
  const previousStatus = React.useRef(step.status);
  const animateCompletion = previousStatus.current !== 'completed' && step.status === 'completed';

  React.useEffect(() => {
    previousStatus.current = step.status;
  }, [step.status]);

  return (
    <li
      className={cn(
        'relative grid min-h-10 grid-cols-[20px_minmax(0,1fr)] items-center gap-2.5 rounded-lg px-2 py-2 text-sm',
        step.status === 'processing' && 'bg-hover',
      )}
    >
      <StepNode status={step.status} animateCompletion={animateCompletion} />
      {nextStep && (
        <span
          data-plan-step-connector="true"
          aria-hidden="true"
          className="absolute -bottom-2 left-[17.5px] top-8 w-px rounded-full bg-border"
        />
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
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
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

export const PlanIndicator: React.FC<PlanIndicatorProps> = ({ plan, open, onOpenChange }) => {
  const dismissedByPointerRef = React.useRef(false);
  const hasPlan = Boolean(plan && plan.steps.length > 0);
  const total = plan?.steps.length ?? 0;
  const completed = plan?.steps.filter((step) => step.status === 'completed').length ?? 0;

  return (
    <DropdownMenu open={open} onOpenChange={onOpenChange} modal={false}>
      <span className="relative inline-flex">
        <DropdownMenuTrigger asChild>
          <IconButton
            label={hasPlan ? `查看计划进度 ${completed} / ${total}` : '暂无计划'}
            expandableLabel="计划"
            aria-haspopup="menu"
          >
            <span className="relative inline-flex h-4 w-4 shrink-0">
              <ListChecks className="h-4 w-4" strokeWidth={1.75} />
              {hasPlan && <ToolbarProgressBadge className="text-text-600">{completed}/{total}</ToolbarProgressBadge>}
            </span>
          </IconButton>
        </DropdownMenuTrigger>
      </span>
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
          if (dismissedByPointerRef.current || open === false) event.preventDefault();
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
