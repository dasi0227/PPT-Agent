import { interactionCardClassName, interactionReasonClassName, interactionInsetClassName } from './interactionCardStyles';
import { ShieldPlus } from 'lucide-react';
import { useMemo, useState } from 'react';
import type { RunScope } from '../../api/types';
import { useProjectStore } from '../../stores/projectStore';
import { useRunStore } from '../../stores/runStore';
import { ordinalBySlideId } from '../deck/selectors';
import type { ScopeExpansionItem } from './eventReducer';
import { useActiveThreadId } from './useActiveSession';
import { HumanIntervention, DecisionControls, SubmittedFeedback, decisionFeedback, type DecisionOption } from './HumanIntervention';

function scopePageLabel(scope: RunScope, pageOrdinals: Record<string, number>, hasSnapshot: boolean): string {
  const pageNumbers = [...new Set(scope.slide_ids.map((id) => pageOrdinals[id]).filter((number) => number !== undefined))]
    .sort((a, b) => a - b);
  const missingCount = scope.slide_ids.length - pageNumbers.length;
  const numberedPages = pageNumbers.length ? `第 ${pageNumbers.join('、')} 页` : '';
  const pages = !hasSnapshot ? `${scope.slide_ids.length} 页，页码加载中`
    : missingCount ? `${numberedPages}${numberedPages ? '，' : ''}${missingCount} 页已删除`
      : numberedPages || '暂无页面';
  return scope.include_run_created_slides ? `${pages}（含本任务新增页）` : pages;
}

type ScopeDecision = 'approve' | 'revise' | 'refuse';

export function ScopeExpansionCard({ item }: { item: ScopeExpansionItem }) {
  const threadId = useActiveThreadId();
  const answerScopeExpansion = useRunStore(state => state.answerScopeExpansion);
  const snapshot = useProjectStore(state => state.activeProjectId ? state.contentByProjectId[state.activeProjectId] : undefined);
  const pageOrdinals = useMemo(() => ordinalBySlideId(snapshot?.outline), [snapshot?.outline]);
  const [decision, setDecision] = useState<ScopeDecision | ''>('');
  const [feedback, setFeedback] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const options: DecisionOption<ScopeDecision>[] = [{ value: 'approve', label: '批准', tone: 'success' }];
  if (item.proposedScope.source.kind !== 'all_pages') options.push({ value: 'revise', label: '允许全部页', tone: 'warning' });
  options.push({ value: 'refuse', label: '拒绝', tone: 'danger' });
  const submit = async () => {
    if (!threadId || !item.runId || !decision || submitting || item.answer) return;
    setSubmitting(true);
    const accepted = await answerScopeExpansion(threadId, item.runId, {
      interaction_id: item.interactionId, call_id: item.callId, base_revision: item.baseRevision, decision,
      feedback: decisionFeedback(decision, 'refuse', feedback),
    });
    if (!accepted) setSubmitting(false);
  };
  const summary = item.answer ? item.answer.decision === 'refuse' ? '已拒绝扩大修改范围'
    : item.answer.decision === 'revise' ? '已允许修改全部页' : '已批准扩大修改范围' : '申请扩大修改范围';
  const currentLabel = scopePageLabel(item.currentScope, pageOrdinals, !!snapshot);
  const nextLabel = item.answer?.decision === 'refuse' ? currentLabel
    : item.answer?.appliedScope ? scopePageLabel(item.answer.appliedScope, pageOrdinals, !!snapshot)
      : item.answer?.decision === 'revise' ? '全部页（含本任务新增页）' : scopePageLabel(item.proposedScope, pageOrdinals, !!snapshot);
  return <HumanIntervention id={`scope-expansion-details-${item.interactionId}`} pending={!item.answer} icon={ShieldPlus}
    label={summary} tone={item.answer?.decision === 'refuse' ? 'danger' : 'success'}>
    <article className={`${interactionCardClassName} timeline-detail-card overflow-hidden`}>
      <div className="px-4 pb-4 pt-[15px]">
        <p className={interactionReasonClassName}>{item.reason}</p>
        <div className={`${interactionInsetClassName} flex flex-wrap items-baseline gap-x-2 gap-y-1 text-[13px] leading-5 tabular-nums [&>span]:min-w-0 [&>span]:break-words`} aria-label="修改范围变更">
          <span className="text-text-700"><span className="sr-only">原先：</span>{currentLabel}</span>
          <span className="shrink-0 text-text-600" aria-hidden="true">→</span>
          <span className="text-text-900"><span className="sr-only">{item.answer ? '现在：' : '扩展后：'}</span>{nextLabel}</span>
        </div>
      </div>
      {item.answer ? <SubmittedFeedback feedback={item.answer.feedback} /> : <DecisionControls options={options}
        selected={decision} onSelect={setDecision} onSubmit={() => void submit()} busy={submitting}
        feedback={feedback} onFeedback={decision === 'refuse' ? setFeedback : undefined} label="范围扩权操作" />}
    </article>
  </HumanIntervention>;
}
