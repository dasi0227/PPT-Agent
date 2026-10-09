import { useState } from 'react';
import { ArrowRight, Copy, ListChecks } from 'lucide-react';
import { runsApi } from '../../api/runs';
import { cn } from '../../lib/utils';
import { MarkdownMessage } from './MarkdownMessage';
import { LongContent } from './LongContent';
import { TimelineCardHeader, timelineCardActionClass } from './TimelineCardHeader';
import { showGlobalError, showGlobalSuccess } from '../../stores/toastStore';
import type { PlanApprovalItem } from './eventReducer';
import { TimelineChevron } from './TimelineDisclosure';

const decisions = [
  ['approve', '批准执行'],
  ['revise', '返回修改'],
  ['refuse', '拒绝计划'],
] as const;

type PlanDecision = typeof decisions[number][0];

const answeredEventText: Record<PlanDecision, string> = {
  approve: '已批准执行计划',
  revise: '计划已返回修改',
  refuse: '计划已拒绝',
};

function decisionClass(decision: PlanDecision, selected: boolean): string {
  if (!selected) return `border-border text-text-600 ${decision === 'approve' ? 'ui-success hover:border-success/20 focus-visible:border-success/20' : decision === 'revise' ? 'ui-warning hover:border-warning/20 focus-visible:border-warning/20' : 'ui-danger hover:border-danger/20 focus-visible:border-danger/20'}`;
  return decision === 'approve'
    ? 'border-success/20 bg-success-soft text-success'
    : decision === 'revise'
      ? 'border-warning/20 bg-warning-soft text-warning'
      : 'border-danger/20 bg-danger-soft text-danger';
}

function PlanContent({ content }: { content: string }) {
  return (
    <LongContent
      maxHeight={340}
      contentClassName="px-4 pb-[18px] pt-4 text-[13px] leading-[1.85] text-text-700"
      fadeClassName="from-timeline-card/0 via-timeline-card/90 to-timeline-card"
      buttonClassName="bg-timeline-card text-[11px] shadow-none"
      controlsClassName="mt-0 pb-3"
      testId="plan-content-preview"
    >
      <MarkdownMessage content={content} className="text-[13px] leading-[1.85] text-text-700 prose-h1:text-[21px] prose-h1:leading-normal prose-h1:mb-[18px] prose-h2:text-[15px] prose-headings:text-text-900 [&>:first-child]:mt-0" />
    </LongContent>
  );
}

function PlanBody({ item }: { item: PlanApprovalItem }) {
  const title = item.plan.title.trim();
  const hasTitle = /^ {0,3}#\s+\S/m.test(item.plan.content) || item.plan.content.includes(title);
  const content = title && !hasTitle
    ? `# ${title}\n\n${item.plan.content}` : item.plan.content;
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(content);
      showGlobalSuccess('已复制计划');
    } catch {
      showGlobalError('复制失败，请展开计划后手动复制。');
    }
  };
  return (
    <>
      <TimelineCardHeader action={<button type="button" onClick={() => void copy()} className={timelineCardActionClass}>
        <Copy className="h-3.5 w-3.5" strokeWidth={1.75} aria-hidden="true" />复制
      </button>}>
        <span className="flex min-w-0 items-center gap-2 pt-1.5 text-xs font-normal text-text-900">
          <ListChecks className="h-4 w-4 shrink-0 text-success" strokeWidth={1.75} aria-hidden="true" />计划
        </span>
      </TimelineCardHeader>
      <PlanContent content={content} />
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
        className="timeline-disclosure-trigger flex min-h-8 w-full items-center gap-2 px-1.5 py-1 text-left text-[13px] font-normal leading-5 text-text-900 focus-visible:outline-none"
      >
        <ListChecks className={cn(
          'h-4 w-4 shrink-0',
          decision === 'approve' ? 'text-success' : decision === 'revise' ? 'text-warning' : 'text-danger',
        )} strokeWidth={1.75} />
        <span className="min-w-0 flex-1 truncate">{answeredEventText[decision]}</span>
        <TimelineChevron open={expanded} />
      </button>
      {expanded && (
        <article
          id={detailsId}
          data-testid="answered-plan-card"
          className="timeline-detail-card rounded-[10px] overflow-hidden border border-border bg-timeline-card"
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
  const canSubmit = !submitting && !!decision;
  const submit = async () => {
    if (!canSubmit || !item.runId) return;
    setSubmitting(true);
    try { await runsApi.submitPlanApproval(item.runId, { interaction_id: item.interactionId, plan_id: item.plan.id, decision: decision as 'approve' | 'revise' | 'refuse', feedback: feedback, idempotency_key: item.interactionId }); }
    catch { setSubmitting(false); }
  };
  if (answered) return <AnsweredPlanApproval item={item} decision={answered.decision} />;
  return <article className="rounded-[10px] overflow-hidden border border-border bg-timeline-card">
    <PlanBody item={item} />
    <div className="mx-3.5 border-t border-border py-3" role="group" aria-label="计划处理方式" data-testid="plan-approval-actions">
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
      {decision === 'revise' && <textarea className="mt-3 min-h-24 w-full rounded-lg border border-border px-3 py-2 text-sm bg-surface focus:outline-none" value={feedback} onChange={(event) => setFeedback(event.target.value)} placeholder="说明需要调整的内容" />}
      <div className="mt-3 flex justify-end"><button type="button" disabled={!canSubmit} onClick={() => void submit()} className="inline-flex h-9 items-center gap-1 rounded-lg border border-accent/20 bg-accent-soft text-selected-foreground ui-interactive px-3 text-sm transition-colors disabled:cursor-not-allowed disabled:opacity-40"><ArrowRight className="h-4 w-4" strokeWidth={1.75} />{submitting ? '提交中' : '继续'}</button></div>
    </div>
  </article>;
}
