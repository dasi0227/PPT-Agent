import { useLayoutEffect, useRef, useState } from 'react';
import { ChevronDown, ChevronRight, ChevronUp, ListChecks, Send } from 'lucide-react';
import { runsApi } from '../../api/runs';
import { MarkdownMessage } from './MarkdownMessage';
import type { PlanApprovalItem } from './eventReducer';

const decisions = [
  ['approve', '批准执行'],
  ['revise', '返回修改'],
  ['cancel', '取消停止'],
] as const;

type PlanDecision = typeof decisions[number][0];

function decisionLabel(decision: PlanDecision): string {
  return decisions.find(([value]) => value === decision)?.[1] ?? '';
}

function decisionClass(decision: PlanDecision, selected: boolean): string {
  if (!selected) return 'border-border text-text-600';
  return decision === 'cancel'
    ? 'border-danger/40 bg-danger/10 text-danger'
    : 'border-accent/40 bg-accent-soft text-accent';
}

function PlanContentPreview({ content }: { content: string }) {
  const [expanded, setExpanded] = useState(false);
  const [overflowing, setOverflowing] = useState(false);
  const contentRef = useRef<HTMLDivElement>(null);

  useLayoutEffect(() => {
    if (expanded) return undefined;
    const element = contentRef.current;
    if (!element) return undefined;
    const measure = () => setOverflowing(element.scrollHeight - element.clientHeight > 1);
    measure();
    if (typeof ResizeObserver === 'undefined') return undefined;
    const observer = new ResizeObserver(measure);
    observer.observe(element);
    return () => observer.disconnect();
  }, [content, expanded]);

  return (
    <div className="mt-3">
      <div
        ref={contentRef}
        data-testid="plan-content-preview"
        className={expanded ? '' : 'relative max-h-[480px] overflow-hidden'}
      >
        <div className="text-sm leading-6 text-text-700">
          <MarkdownMessage content={content} />
        </div>
        {!expanded && overflowing && (
          <div className="absolute inset-x-0 bottom-0 flex h-16 items-end justify-center bg-gradient-to-b from-surface/0 via-surface/85 to-surface pb-1 backdrop-blur-[1px]">
            <button
              type="button"
              onClick={() => setExpanded(true)}
              className="inline-flex h-7 items-center rounded-full border border-border bg-surface px-3 text-xs font-medium text-text-700 shadow-sm hover:bg-panel-muted hover:text-text-900"
            >
              展开全部
            </button>
          </div>
        )}
      </div>
      {expanded && overflowing && (
        <div className="mt-2 flex justify-center">
          <button
            type="button"
            onClick={() => setExpanded(false)}
            className="inline-flex h-7 items-center gap-1 rounded-md px-2 text-xs text-text-400 hover:bg-panel-muted hover:text-text-700"
          >
            <ChevronUp className="h-3.5 w-3.5" strokeWidth={1.75} />
            收起
          </button>
        </div>
      )}
    </div>
  );
}

function PlanBody({
  item,
  statusText = '等待确认',
  preview = false,
}: {
  item: PlanApprovalItem;
  statusText?: string;
  preview?: boolean;
}) {
  return <>
    <div className="flex items-center gap-2"><ListChecks className="h-5 w-5 text-accent" /><div><h3 className="text-sm font-semibold text-text-900">{item.plan.title}</h3><p className="text-xs text-text-400">{item.plan.steps.length} 个步骤 · {statusText}</p></div></div>
    {preview
      ? <PlanContentPreview content={item.plan.content} />
      : <div className="mt-3 text-sm leading-6 text-text-700"><MarkdownMessage content={item.plan.content} /></div>}
  </>;
}

function AnsweredPlanApproval({ item, decision, feedback }: { item: PlanApprovalItem; decision: PlanDecision; feedback?: string }) {
  const [expanded, setExpanded] = useState(false);
  const label = decisionLabel(decision);
  return <article className="overflow-hidden rounded-[10px] border border-border-strong bg-surface">
    <button type="button" aria-expanded={expanded} onClick={() => setExpanded((value) => !value)} className="flex min-h-10 w-full items-center gap-2 px-3 py-2 text-left text-sm text-text-600">
      <ListChecks className="h-4 w-4 shrink-0 text-accent" />
      <span className="min-w-0 flex-1 truncate">计划已{label}</span>
      {expanded ? <ChevronDown className="h-3.5 w-3.5 shrink-0 text-text-400" /> : <ChevronRight className="h-3.5 w-3.5 shrink-0 text-text-400" />}
    </button>
    {expanded && <div className="border-t border-border p-4">
      <PlanBody item={item} statusText={`已${label}`} />
      <div className="mt-4 border-t border-border pt-3" role="group" aria-label="已提交的计划处理方式">
        <div className="grid grid-cols-3 gap-2">
          {decisions.map(([value, optionLabel]) => <div key={value} aria-pressed={decision === value} className={`flex items-center justify-center rounded-lg border px-2 py-2 text-xs font-medium ${decisionClass(value, decision === value)}`}>{optionLabel}</div>)}
        </div>
        {decision === 'revise' && <p className="mt-3 rounded-lg border border-border bg-panel-muted px-3 py-2 text-sm leading-6 text-text-700"><span className="mr-2 font-medium text-text-900">修改反馈</span>{feedback}</p>}
      </div>
    </div>}
  </article>;
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
    try { await runsApi.submitPlanApproval(item.runId, { interaction_id: item.interactionId, plan_id: item.plan.id, expected_revision: item.plan.revision, decision: decision as 'approve' | 'revise' | 'cancel', feedback: feedback.trim(), idempotency_key: `${item.interactionId}:${item.plan.revision}` }); }
    catch { setSubmitting(false); }
  };
  if (answered) return <AnsweredPlanApproval item={item} decision={answered.decision} feedback={answered.feedback} />;
  return <article className="rounded-[10px] border border-border-strong bg-surface p-4">
    <PlanBody item={item} preview />
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
      {decision === 'revise' && <textarea className="mt-3 min-h-24 w-full rounded-lg border border-border px-3 py-2 text-sm focus:border-accent focus:outline-none" value={feedback} onChange={(event) => setFeedback(event.target.value)} placeholder="说明需要调整的内容" required />}
      <div className="mt-3 flex justify-end"><button type="button" disabled={!canSubmit} onClick={() => void submit()} className="inline-flex items-center gap-1 rounded-lg bg-accent px-3 py-2 text-xs font-semibold text-white disabled:cursor-not-allowed disabled:opacity-35"><Send className="h-3.5 w-3.5" />{submitting ? '提交中' : '提交'}</button></div>
    </div>
  </article>;
}
