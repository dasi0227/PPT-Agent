import { useEffect, useState, type ReactNode } from 'react';
import type { LucideIcon } from 'lucide-react';
import { cn } from '../../lib/utils';
import { TimelineChevron, TimelineDisclosure } from './TimelineDisclosure';

export function HumanInterventionBadge() {
  return <span className="shrink-0 rounded bg-warning-soft px-1.5 py-0.5 text-[10px] font-semibold text-[rgb(var(--ui-warning-foreground))]">待人工介入</span>;
}

export function HumanIntervention({ id, pending, icon: Icon, label, tone = 'success', children }: {
  id: string; pending: boolean; icon: LucideIcon; label: string;
  tone?: 'success' | 'warning' | 'danger'; children: ReactNode;
}) {
  const [expanded, setExpanded] = useState(pending);
  useEffect(() => { setExpanded(pending); }, [pending]);
  return <div className="min-w-0 rounded-lg">
    <button type="button" aria-expanded={expanded} aria-controls={id} onClick={() => setExpanded(value => !value)}
      className="timeline-disclosure-trigger ui-interactive flex min-h-8 w-full items-center gap-2 rounded-md px-1.5 py-1 text-left text-[13px] font-normal leading-5 text-text-900">
      <Icon className={cn('h-4 w-4 shrink-0', pending || tone === 'warning' ? 'text-warning' : tone === 'danger' ? 'text-danger' : 'text-success')} strokeWidth={1.75} aria-hidden="true" />
      <span className="flex min-w-0 flex-1 items-center gap-2"><span className="min-w-0 truncate">{label}</span>{pending && <HumanInterventionBadge />}</span>
      <TimelineChevron open={expanded} />
    </button>
    <TimelineDisclosure id={id} open={expanded}>{children}</TimelineDisclosure>
  </div>;
}

export interface DecisionOption<T extends string> {
  value: T;
  label: string;
  tone: 'success' | 'warning' | 'danger';
}

const selectedClass = {
  success: 'ui-success border-success/20 bg-success-soft text-success',
  warning: 'ui-warning border-warning/20 bg-warning-soft text-[rgb(var(--ui-warning-foreground))]',
  danger: 'ui-danger border-danger/20 bg-danger-soft text-danger',
};

export function DecisionControls<T extends string>({ options, selected, onSelect, onSubmit, busy, disabled, feedback, onFeedback, error, label, testId, inset = false }: {
  options: readonly DecisionOption<T>[]; selected: T | ''; onSelect: (value: T) => void;
  onSubmit: () => void; busy: boolean; disabled?: boolean;
  feedback?: string; onFeedback?: (value: string) => void; error?: string; label: string; testId?: string; inset?: boolean;
}) {
  return <div role="group" aria-label={label} data-testid={testId} className={inset ? undefined : 'border-t border-border px-3.5 py-3'}>
    <div className="grid gap-2" style={{ gridTemplateColumns: `repeat(${options.length}, minmax(0, 1fr))` }}>
      {options.map(option => <button key={option.value} type="button" aria-pressed={selected === option.value} disabled={busy}
        onClick={() => onSelect(option.value)}
        className={cn('min-w-0 rounded-lg border px-2 py-2 text-xs font-medium transition-colors focus-visible:underline disabled:cursor-not-allowed disabled:opacity-40',
          selected === option.value ? selectedClass[option.tone] : 'ui-interactive border-border text-text-600')}>
        {option.label}
      </button>)}
    </div>
    {onFeedback && <textarea aria-label="拒绝原因或修改建议" placeholder="拒绝原因或修改建议" value={feedback ?? ''}
      onChange={event => onFeedback(event.target.value)} disabled={busy}
      className="mt-3 min-h-24 w-full resize-y rounded-lg border border-border bg-surface px-3 py-2 text-sm text-text-900 focus:outline-none disabled:opacity-40" />}
    {error && <p role="alert" className="mt-2 text-xs text-danger">{error}</p>}
    <div className="mt-3 flex justify-end"><button type="button" onClick={onSubmit} disabled={busy || disabled || !selected}
      className="ui-primary inline-flex h-9 items-center justify-center rounded-lg px-3 text-xs transition-colors disabled:cursor-not-allowed disabled:opacity-40">
      {busy ? '提交中' : '继续'}
    </button></div>
  </div>;
}

export function SubmittedFeedback({ feedback }: { feedback?: string }) {
  return feedback?.trim() ? <p className="px-3.5 py-3 whitespace-pre-wrap break-words text-xs leading-5 text-text-700">{feedback}</p> : null;
}

export function decisionFeedback(selected: string, rejection: string, feedback: string) {
  return selected === rejection && feedback.trim() ? feedback : undefined;
}
