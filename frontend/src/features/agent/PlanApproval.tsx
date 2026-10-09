import { useState } from 'react';
import { Copy, ListChecks } from 'lucide-react';
import { runsApi } from '../../api/runs';
import { MarkdownMessage } from './MarkdownMessage';
import { LongContent } from './LongContent';
import { TimelineCardHeader, timelineCardActionClass } from './TimelineCardHeader';
import { showGlobalError, showGlobalSuccess } from '../../stores/toastStore';
import type { PlanApprovalItem } from './eventReducer';
import { HumanIntervention, DecisionControls, SubmittedFeedback, decisionFeedback } from './HumanIntervention';

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
        {null}
      </TimelineCardHeader>
      <PlanContent content={content} />
    </>
  );
}

export function PlanApproval({ item }: { item: PlanApprovalItem }) {
  const [decision, setDecision] = useState<'approve' | 'refuse' | ''>('');
  const [feedback, setFeedback] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState('');
  const submit = async () => {
    if (submitting || !decision || !item.runId || item.answer) return;
    setSubmitting(true); setError('');
    try {
      await runsApi.submitPlanApproval(item.runId, {
        interaction_id: item.interactionId, plan_id: item.plan.id, decision,
        feedback: decisionFeedback(decision, 'refuse', feedback), idempotency_key: item.interactionId,
      });
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '审批提交失败'); setSubmitting(false);
    }
  };
  return <HumanIntervention id={`plan-approval-details-${item.interactionId}`} pending={!item.answer} icon={ListChecks}
    label={item.answer ? item.answer.decision === 'approve' ? '已批准执行计划' : '计划已拒绝' : '审批计划'}
    tone={item.answer?.decision === 'refuse' ? 'danger' : 'success'}>
    <article data-testid={item.answer ? 'answered-plan-card' : undefined} className="timeline-detail-card rounded-[10px] overflow-hidden border border-border bg-timeline-card">
      <PlanBody item={item} />
      {item.answer ? <SubmittedFeedback feedback={item.answer.feedback} /> : <DecisionControls
        options={[{ value: 'approve', label: '批准', tone: 'success' }, { value: 'refuse', label: '拒绝', tone: 'danger' }]}
        selected={decision} onSelect={setDecision} onSubmit={() => void submit()} busy={submitting}
        feedback={feedback} onFeedback={decision === 'refuse' ? setFeedback : undefined} error={error}
        label="计划处理方式" testId="plan-approval-actions" />}
    </article>
  </HumanIntervention>;
}
