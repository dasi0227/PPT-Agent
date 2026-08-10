import { useState } from 'react';
import { CheckCircle2, ListChecks, Send } from 'lucide-react';
import { runsApi } from '../../api/runs';
import { MarkdownMessage } from './MarkdownMessage';
import type { PlanApprovalItem } from './eventReducer';

export function PlanApproval({ item }: { item: PlanApprovalItem }) {
  const [decision, setDecision] = useState<'approve' | 'revise' | 'cancel' | ''>('');
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
  if (answered) return <div className="flex items-center gap-2 rounded-lg border border-border bg-surface px-3 py-2 text-sm text-text-600"><CheckCircle2 className="h-4 w-4 text-success" /><span>计划已{answered.decision === 'approve' ? '批准执行' : answered.decision === 'revise' ? '返回修改' : '取消停止'}</span></div>;
  return <article className="rounded-[10px] border border-border-strong bg-surface p-4">
    <div className="flex items-center gap-2"><ListChecks className="h-5 w-5 text-accent" /><div><h3 className="text-sm font-semibold text-text-900">{item.plan.title}</h3><p className="text-xs text-text-400">{item.plan.steps.length} 个步骤 · 等待确认</p></div></div>
    <div className="mt-3 text-sm leading-6 text-text-700"><MarkdownMessage content={item.plan.content} /></div>
    <div className="mt-3 grid grid-cols-3 gap-2" role="radiogroup" aria-label="计划处理方式">
      {([['approve','批准执行'], ['revise','返回修改'], ['cancel','取消停止']] as const).map(([value, label]) => <label key={value} className={`flex cursor-pointer items-center justify-center rounded-lg border px-2 py-2 text-xs font-medium ${decision === value ? value === 'cancel' ? 'border-danger/40 bg-danger/10 text-danger' : 'border-accent/40 bg-accent-soft text-accent' : 'border-border text-text-600'}`}><input className="sr-only" type="radio" name={item.interactionId} checked={decision === value} onChange={() => setDecision(value)} /><span>{label}</span></label>)}
    </div>
    {decision === 'revise' && <textarea className="mt-3 min-h-24 w-full rounded-lg border border-border px-3 py-2 text-sm focus:border-accent focus:outline-none" value={feedback} onChange={(event) => setFeedback(event.target.value)} placeholder="说明需要调整的内容" required />}
    <div className="mt-3 flex justify-end"><button type="button" disabled={!canSubmit} onClick={() => void submit()} className="inline-flex items-center gap-1 rounded-lg bg-accent px-3 py-2 text-xs font-semibold text-white disabled:cursor-not-allowed disabled:opacity-35"><Send className="h-3.5 w-3.5" />{submitting ? '提交中' : '提交'}</button></div>
  </article>;
}
