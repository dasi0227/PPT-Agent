import { Check, ChevronRight, ShieldPlus, TriangleAlert, X } from 'lucide-react';
import { useMemo, useState } from 'react';
import type { RunScope } from '../../api/types';
import { useProjectStore } from '../../stores/projectStore';
import { useRunStore } from '../../stores/runStore';
import { ordinalBySlideId } from '../deck/selectors';
import type { ScopeExpansionItem } from './eventReducer';
import { TimelineDisclosure } from './TimelineDisclosure';
import { useActiveThreadId } from './useActiveSession';

function scopePageLabel(scope: RunScope, pageOrdinals: Record<string, number>, hasSnapshot: boolean): string {
  if (scope.source.kind === 'all_pages') return '全部页';
  if (!hasSnapshot) return '页码加载中';

  const pageNumbers = [...new Set(scope.slide_ids.map((id) => pageOrdinals[id]).filter((number) => number !== undefined))]
    .sort((a, b) => a - b);
  const missingCount = scope.slide_ids.length - pageNumbers.length;
  const numberedPages = pageNumbers.length ? `第 ${pageNumbers.join('、')} 页` : '';
  if (!missingCount) return numberedPages || '暂无页面';
  return `${numberedPages}${numberedPages ? '，' : ''}${missingCount} 页已删除`;
}

const actionClassName = 'inline-flex min-h-[31px] items-center justify-center gap-[5px] rounded-md border border-transparent px-[11px] text-xs font-semibold transition-colors focus-visible:outline-none focus-visible:underline focus-visible:underline-offset-[3px] disabled:cursor-not-allowed disabled:opacity-50';

export function ScopeExpansionCard({ item }: { item: ScopeExpansionItem }) {
  const threadId = useActiveThreadId();
  const answerScopeExpansion = useRunStore((state) => state.answerScopeExpansion);
  const snapshot = useProjectStore((state) => state.activeProjectId ? state.contentByProjectId[state.activeProjectId] : undefined);
  const [submitting, setSubmitting] = useState<'approve' | 'reject' | 'adjust' | null>(null);
  const [expanded, setExpanded] = useState(false);
  const pageOrdinals = useMemo(() => ordinalBySlideId(snapshot?.outline), [snapshot?.outline]);

  if (item.answer) {
    const decision = item.answer.decision;
    const accepted = decision !== 'reject';
    const summary = decision === 'reject' ? '已拒绝扩大修改范围'
      : decision === 'adjust' ? '已允许修改全部页' : '已批准扩大修改范围';
    const currentLabel = scopePageLabel(item.currentScope, pageOrdinals, !!snapshot);
    const nextLabel = decision === 'reject' ? currentLabel
      : item.answer.appliedScope ? scopePageLabel(item.answer.appliedScope, pageOrdinals, !!snapshot)
        : decision === 'adjust' ? '全部页' : scopePageLabel(item.proposedScope, pageOrdinals, !!snapshot);
    const detailsId = `scope-expansion-details-${item.interactionId}`;
    return (
      <div>
        <button
          type="button"
          aria-expanded={expanded}
          aria-controls={detailsId}
          onClick={() => setExpanded((value) => !value)}
          className="ui-interactive flex min-h-[34px] w-full items-center gap-2 rounded-md px-1.5 py-1 text-left text-[13px] leading-5 text-text-600 focus-visible:outline-none"
        >
          <ShieldPlus className={`h-4 w-4 shrink-0 ${accepted ? 'text-success' : 'text-danger'}`} strokeWidth={1.75} aria-hidden="true" />
          <span className="min-w-0 flex-1 truncate">{summary}</span>
          <ChevronRight className={`h-3.5 w-3.5 shrink-0 text-text-400 transition-transform ${expanded ? 'rotate-90' : ''}`} strokeWidth={1.75} aria-hidden="true" />
        </button>
        <TimelineDisclosure open={expanded}>
          <dl id={detailsId} className="grid gap-y-1 pb-2 pl-[30px] pr-2 pt-1 text-xs leading-5 text-text-600">
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

  const submit = async (decision: 'approve' | 'reject' | 'adjust') => {
    if (!threadId || !item.runId || submitting) return;
    setSubmitting(decision);
    const accepted = await answerScopeExpansion(threadId, item.runId, {
      interaction_id: item.interactionId,
      call_id: item.callId,
      base_revision: item.baseRevision,
      decision,
      ...(decision === 'adjust' ? { adjusted_scope: { selection: { kind: 'all_pages' as const } } } : {}),
    });
    if (!accepted) setSubmitting(null);
  };

  return (
    <article className="overflow-hidden rounded-[10px] border border-border bg-surface">
      <div className="px-4 pb-4 pt-[15px] max-[420px]:px-[13px] max-[420px]:pt-[13px]">
        <h3 className="flex items-center gap-[9px] text-[13px] font-semibold leading-5 text-text-900">
          <ShieldPlus className="h-[18px] w-[18px] shrink-0 text-accent" strokeWidth={1.75} aria-hidden="true" />
          Dasi 申请扩大修改范围
        </h3>
        <p className="ml-[27px] mt-[7px] break-words text-xs leading-[19px] text-text-700 max-[420px]:ml-0">{item.reason}</p>
        <dl className="ml-[27px] mt-5 grid gap-y-[9px] text-[13px] leading-5 max-[420px]:ml-0">
          <div className="grid min-w-0 grid-cols-[44px_minmax(0,1fr)] items-baseline gap-x-2">
            <dt className="text-text-600">当前</dt>
            <dd className="min-w-0 break-words font-medium tabular-nums text-text-700">{scopePageLabel(item.currentScope, pageOrdinals, !!snapshot)}</dd>
          </div>
          <div className="grid min-w-0 grid-cols-[44px_minmax(0,1fr)] items-baseline gap-x-2">
            <dt className="text-text-600">扩展</dt>
            <dd className="min-w-0 break-words font-medium tabular-nums text-text-900">{scopePageLabel(item.proposedScope, pageOrdinals, !!snapshot)}</dd>
          </div>
        </dl>
      </div>
      <div className="flex flex-wrap justify-end gap-2 border-t border-border px-[13px] py-[10px]" role="group" aria-label="范围扩权操作">
        <button type="button" disabled={submitting !== null} onClick={() => void submit('reject')} className={`${actionClassName} bg-danger-soft text-[rgb(var(--ui-danger-hover))] hover:border-danger/25 focus-visible:border-danger/25`}>
          <X className="h-[13px] w-[13px] shrink-0" strokeWidth={1.75} aria-hidden="true" />{submitting === 'reject' ? '提交中' : '拒绝'}
        </button>
        {item.proposedScope.source.kind !== 'all_pages' && (
          <button type="button" disabled={submitting !== null} onClick={() => void submit('adjust')} className={`${actionClassName} bg-warning-soft text-[rgb(var(--ui-warning-foreground))] hover:border-warning/25 focus-visible:border-warning/25`}>
            <TriangleAlert className="h-[13px] w-[13px] shrink-0" strokeWidth={1.75} aria-hidden="true" />{submitting === 'adjust' ? '提交中' : '允许全部页'}
          </button>
        )}
        <button type="button" disabled={submitting !== null} onClick={() => void submit('approve')} className={`${actionClassName} bg-success-soft text-[rgb(var(--ui-success-hover))] hover:border-success/25 focus-visible:border-success/25`}>
          <Check className="h-[13px] w-[13px] shrink-0" strokeWidth={1.75} aria-hidden="true" />{submitting === 'approve' ? '提交中' : '批准'}
        </button>
      </div>
    </article>
  );
}
