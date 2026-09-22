import { useState } from 'react';
import { ArrowRight, ChevronRight, ListChecks } from 'lucide-react';
import { runsApi } from '../../api/runs';
import { cn } from '../../lib/utils';
import { MarkdownMessage } from './MarkdownMessage';
import { LongContent } from './LongContent';
import type { PlanApprovalItem } from './eventReducer';

const decisions = [
  ['approve', '批准执行'],
  ['revise', '返回修改'],
  ['cancel', '取消停止'],
] as const;

type PlanDecision = typeof decisions[number][0];

const answeredEventText: Record<PlanDecision, string> = {
  approve: '计划已批准执行',
  revise: '计划已返回修改',
  cancel: '计划已取消',
};

function decisionClass(decision: PlanDecision, selected: boolean): string {
  if (!selected) return 'border-border text-text-600';
  return decision === 'approve'
    ? 'border-success/20 bg-success-soft text-success'
    : decision === 'revise'
      ? 'border-warning/20 bg-warning-soft text-warning'
      : 'border-danger/20 bg-danger-soft text-danger';
}

function PlanContent({ content }: { content: string }) {
  return (
    <LongContent
      className="mt-3"
      contentClassName="text-sm leading-6 text-text-700"
      testId="plan-content-preview"
    >
      <MarkdownMessage content={content} />
    </LongContent>
  );
}

function PlanBody({ item }: { item: PlanApprovalItem }) {
  return (
    <>
      <div className="flex items-center gap-2">
        <ListChecks className="h-5 w-5 shrink-0 text-accent" strokeWidth={1.75} />
        <h3 className="min-w-0 text-sm font-semibold leading-5 text-text-900">{item.plan.title}</h3>
      </div>
      <PlanContent content={item.plan.content} />
    </>
  );
}

function AnsweredPlanApproval({ item, decision }: { item: PlanApprovalItem; decision: PlanDecision }) {
  const [expanded, setExpanded] = useState(false);
  const detailsId = `plan-approval-details-${item.interactionId}`;
  return (
    <div className="rounded-lg">
      <button
        type="button"
        aria-expanded={expanded}
        aria-controls={detailsId}
        onClick={() => setExpanded((value) => !value)}
        className="flex min-h-8 w-full items-center gap-2 px-1.5 py-1 text-left text-[13px] font-normal leading-5 text-text-900 focus-visible:outline-none"
      >
        <ListChecks className="h-4 w-4 shrink-0 text-accent" strokeWidth={1.75} />
        <span className="min-w-0 flex-1 truncate">{answeredEventText[decision]}</span>
        <ChevronRight
          className={cn('h-3.5 w-3.5 shrink-0 text-text-400 transition-transform', expanded && 'rotate-90')}
          strokeWidth={1.75}
          aria-hidden="true"
        />
      </button>
      {expanded && (
        <article
          id={detailsId}
          data-testid="answered-plan-card"
          className="timeline-detail-card rounded-[10px] border border-border-strong bg-surface p-4"
        >
          <PlanBody item={item} />
        </article>
      )}
    </div>
  );
}

export function PlanApproval({ item }: { item: PlanApprovalItem }) {
  const [decision, setDecision] = useState<PlanDecision | ''>('');
  const [feedback, setFeedback] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const answered = item.answer;
  const canSubmit = !submitting && !!decision && (decision !== 'revise' || feedback.trim() !== '');
  const submit = async () => {
    if (!canSubmit || !item.runId) return;
    setSubmitting(true);
    try { await runsApi.submitPlanApproval(item.runId, { interaction_id: item.interactionId, plan_id: item.plan.id, decision: decision as 'approve' | 'revise' | 'cancel', feedback: feedback.trim(), idempotency_key: item.interactionId }); }
    catch { setSubmitting(false); }
  };
  if (answered) return <AnsweredPlanApproval item={item} decision={answered.decision} />;
  return <article className="rounded-[10px] border border-border-strong bg-surface p-4">
    <PlanBody item={item} />
    <div className="mt-4 border-t border-border pt-3" role="group" aria-label="计划处理方式" data-testid="plan-approval-actions">
      <div className="grid grid-cols-3 gap-2">
        {decisions.map(([value, label]) => (
          <button
            key={value}
            type="button"
            aria-pressed={decision === value}
            onClick={(event) => {
              event.stopPropagation();
              setDecision(value);
            }}
            className={`flex items-center justify-center rounded-lg border px-2 py-2 text-xs font-medium ${decisionClass(value, decision === value)}`}
          >
            {label}
          </button>
        ))}
      </div>
      {decision === 'revise' && <textarea className="mt-3 min-h-24 w-full rounded-lg border border-border px-3 py-2 text-sm focus:border-2 focus:border-ink focus:outline-none focus-visible:ring-0 focus-visible:ring-offset-0" value={feedback} onChange={(event) => setFeedback(event.target.value)} placeholder="说明需要调整的内容" required />}
      <div className="mt-3 flex justify-end"><button type="button" disabled={!canSubmit} onClick={() => void submit()} className="inline-flex h-9 items-center gap-1 rounded-lg bg-accent-soft px-3 text-sm text-accent transition-colors hover:bg-accent-soft disabled:cursor-not-allowed disabled:opacity-40"><ArrowRight className="h-4 w-4" strokeWidth={1.75} />{submitting ? '提交中' : '继续'}</button></div>
    </div>
  </article>;
}
