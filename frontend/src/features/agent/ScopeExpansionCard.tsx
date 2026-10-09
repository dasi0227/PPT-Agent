import { interactionCardClassName, interactionTitleClassName, interactionReasonClassName, interactionInsetClassName, timelineDetailCardClassName } from './interactionCardStyles';
import { Check, ShieldPlus, TriangleAlert, X } from 'lucide-react';
import { useMemo, useState } from 'react';
import type { RunScope } from '../../api/types';
import { useProjectStore } from '../../stores/projectStore';
import { useRunStore } from '../../stores/runStore';
import { ordinalBySlideId } from '../deck/selectors';
import type { ScopeExpansionItem } from './eventReducer';
import { TimelineDisclosure, TimelineChevron } from './TimelineDisclosure';
import { useActiveThreadId } from './useActiveSession';

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

const actionClassName = 'inline-flex min-h-[31px] items-center justify-center gap-[5px] rounded-md border px-[11px] text-xs font-semibold transition-colors focus-visible:outline-none focus-visible:underline focus-visible:underline-offset-[3px] disabled:cursor-not-allowed disabled:opacity-50';

export function ScopeExpansionCard({ item }: { item: ScopeExpansionItem }) {
  const threadId = useActiveThreadId();
  const answerScopeExpansion = useRunStore((state) => state.answerScopeExpansion);
  const snapshot = useProjectStore((state) => state.activeProjectId ? state.contentByProjectId[state.activeProjectId] : undefined);
  const [submitting, setSubmitting] = useState<'approve' | 'refuse' | 'revise' | null>(null);
  const [expanded, setExpanded] = useState(false);
  const pageOrdinals = useMemo(() => ordinalBySlideId(snapshot?.outline), [snapshot?.outline]);

  if (item.answer) {
    const decision = item.answer.decision;
    const accepted = decision !== 'refuse';
    const summary = decision === 'refuse' ? '已拒绝扩大修改范围'
      : decision === 'revise' ? '已允许修改全部页' : '已批准扩大修改范围';
    const currentLabel = scopePageLabel(item.currentScope, pageOrdinals, !!snapshot);
    const nextLabel = decision === 'refuse' ? currentLabel
      : item.answer.appliedScope ? scopePageLabel(item.answer.appliedScope, pageOrdinals, !!snapshot)
        : decision === 'revise' ? '全部页' : scopePageLabel(item.proposedScope, pageOrdinals, !!snapshot);
    const detailsId = `scope-expansion-details-${item.interactionId}`;
    return (
      <div>
        <button
          type="button"
          aria-expanded={expanded}
          aria-controls={detailsId}
          onClick={() => setExpanded((value) => !value)}
          className="timeline-disclosure-trigger ui-interactive flex min-h-[34px] w-full items-center gap-2 rounded-md px-1.5 py-1 text-left text-[13px] leading-5 text-text-600 focus-visible:outline-none"
        >
          <ShieldPlus className={`h-4 w-4 shrink-0 ${accepted ? 'text-success' : 'text-danger'}`} strokeWidth={1.75} aria-hidden="true" />
          <span className="min-w-0 flex-1 truncate">{summary}</span>
          <TimelineChevron open={expanded} />
        </button>
        <TimelineDisclosure open={expanded}>
          <dl id={detailsId} className={`${timelineDetailCardClassName} timeline-detail-card grid gap-y-1 px-3.5 py-3 text-xs leading-5 text-text-600`}>
            <div className="grid min-w-0 grid-cols-[44px_minmax(0,1fr)] gap-x-2">
              <dt>原先：</dt><dd className="min-w-0 break-words tabular-nums">{currentLabel}</dd>
            </div>
            <div className="grid min-w-0 grid-cols-[44px_minmax(0,1fr)] gap-x-2">
              <dt>现在：</dt><dd className="min-w-0 break-words font-medium tabular-nums text-text-900">{nextLabel}</dd>
            </div>
          </dl>
        </TimelineDisclosure>
      </div>
    );
  }

  const submit = async (decision: 'approve' | 'refuse' | 'revise') => {
    if (!threadId || !item.runId || submitting) return;
    setSubmitting(decision);
    const accepted = await answerScopeExpansion(threadId, item.runId, {
      interaction_id: item.interactionId,
      call_id: item.callId,
      base_revision: item.baseRevision,
      decision,
    });
    if (!accepted) setSubmitting(null);
  };

  return (
    <article className={`${interactionCardClassName} overflow-hidden`}>
      <div className="px-4 pb-4 pt-[15px] max-[420px]:px-[13px] max-[420px]:pt-[13px]">
        <h3 className={interactionTitleClassName}>
          申请扩大修改范围
        </h3>
        <p className={interactionReasonClassName}>{item.reason}</p>
        <div className={`${interactionInsetClassName} flex flex-wrap items-baseline gap-x-[9px] gap-y-1 text-[13px] leading-5 tabular-nums [&>span]:min-w-0 [&>span]:break-words`} aria-label="修改范围变更">
          <span className="text-text-700"><span className="sr-only">当前：</span>{scopePageLabel(item.currentScope, pageOrdinals, !!snapshot)}</span>
          <span className="shrink-0 text-text-600" aria-hidden="true">→</span>
          <span className="text-text-900"><span className="sr-only">扩展后：</span>{scopePageLabel(item.proposedScope, pageOrdinals, !!snapshot)}</span>
        </div>
      </div>
      <div className="flex flex-wrap justify-end gap-2 border-t border-border px-[13px] py-[10px]" role="group" aria-label="范围扩权操作">
        {item.proposedScope.source.kind !== 'all_pages' && (
          <button type="button" disabled={submitting !== null} onClick={() => void submit('revise')} className={`${actionClassName} border-warning/20 bg-warning-soft text-[rgb(var(--ui-warning-foreground))] hover:border-warning/25 focus-visible:border-warning/25`}>
            <TriangleAlert className="h-[13px] w-[13px] shrink-0" strokeWidth={1.75} aria-hidden="true" />{submitting === 'revise' ? '提交中' : '允许全部项'}
          </button>
        )}
        <button type="button" disabled={submitting !== null} onClick={() => void submit('refuse')} className={`${actionClassName} border-danger/20 bg-danger-soft text-[rgb(var(--ui-danger-hover))] hover:border-danger/25 focus-visible:border-danger/25`}>
          <X className="h-[13px] w-[13px] shrink-0" strokeWidth={1.75} aria-hidden="true" />{submitting === 'refuse' ? '提交中' : '拒绝'}
        </button>
        <button type="button" disabled={submitting !== null} onClick={() => void submit('approve')} className={`${actionClassName} border-success/20 bg-success-soft text-[rgb(var(--ui-success-hover))] hover:border-success/25 focus-visible:border-success/25`}>
          <Check className="h-[13px] w-[13px] shrink-0" strokeWidth={1.75} aria-hidden="true" />{submitting === 'approve' ? '提交中' : '批准'}
        </button>
      </div>
    </article>
  );
}
