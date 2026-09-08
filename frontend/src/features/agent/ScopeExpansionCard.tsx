import { Check, ShieldPlus, X } from 'lucide-react';
import { useMemo, useState } from 'react';
import { useProjectStore } from '../../stores/projectStore';
import { useRunStore } from '../../stores/runStore';
import { orderedSlides } from '../deck/selectors';
import type { ScopeExpansionItem } from './eventReducer';
import { targetLabel } from './runtimeLabels';
import { useActiveThreadId } from './useActiveSession';

export function ScopeExpansionCard({ item }: { item: ScopeExpansionItem }) {
  const threadId = useActiveThreadId();
  const answerScopeExpansion = useRunStore((state) => state.answerScopeExpansion);
  const snapshot = useProjectStore((state) => state.activeProjectId ? state.contentByProjectId[state.activeProjectId] : undefined);
  const [submitting, setSubmitting] = useState<'approve' | 'reject' | 'adjust' | null>(null);
  const pageLabels = useMemo(() => Object.fromEntries(orderedSlides(snapshot).map((slide, index) => [
    slide.id,
    `第 ${index + 1} 页 · ${slide.title || '未命名页面'}`,
  ])), [snapshot]);
  const requestedPages = item.requestedAddition.slide_ids?.map((id) => pageLabels[id] ?? `已删除页面 · ${id}`) ?? [];

  if (item.answer) {
    const accepted = item.answer.decision !== 'reject';
    const acceptedLabel = item.answer.decision === 'adjust' && !item.answer.appliedScope
      ? '已提交范围调整，Agent 将在更新后的范围内继续'
      : `已更新范围：${targetLabel(item.answer.appliedScope ?? item.proposedScope)}`;
    return (
      <div className="grid min-h-[34px] grid-cols-[18px_minmax(0,1fr)] items-center gap-2 px-1.5 py-1 text-[13px] text-text-600">
        <ShieldPlus className={`h-4 w-4 ${accepted ? 'text-success' : 'text-warning'}`} strokeWidth={1.75} />
        <span>{accepted ? acceptedLabel : '已拒绝范围扩展，Agent 将在原范围内继续'}</span>
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
      ...(decision === 'adjust' ? { adjusted_scope: { object: 'global' as const, selection: { kind: 'all_pages' as const } } } : {}),
    });
    if (!accepted) setSubmitting(null);
  };

  return (
    <article className="overflow-hidden rounded-xl border border-border bg-surface">
      <div className="grid grid-cols-[20px_minmax(0,1fr)] gap-2.5 px-3 pb-3 pt-3">
        <ShieldPlus className="mt-px h-[18px] w-[18px] text-accent" strokeWidth={1.75} aria-hidden="true" />
        <div className="min-w-0">
          <h3 className="text-[13px] font-bold leading-5 text-text-900">Agent 申请扩大修改范围</h3>
          <p className="mt-1 text-xs leading-5 text-text-700">{item.reason}</p>
          <dl className="mt-2 grid grid-cols-[58px_minmax(0,1fr)] gap-x-2 gap-y-1.5 rounded-lg bg-panel-muted px-2.5 py-2 text-xs">
            <dt className="text-text-500">当前</dt><dd className="truncate font-medium text-text-800">{targetLabel(item.currentScope)}</dd>
            <dt className="text-text-500">扩展后</dt><dd className="truncate font-medium text-text-900">{targetLabel(item.proposedScope)} · {item.affectedPageCount} 页</dd>
            {requestedPages.length ? <><dt className="text-text-500">新增页面</dt><dd className="text-text-700">{requestedPages.join('、')}</dd></> : null}
          </dl>
        </div>
      </div>
      <div className="flex flex-wrap justify-end gap-1.5 border-t border-border bg-panel-muted px-3 py-2" role="group" aria-label="范围扩权操作">
        <button type="button" disabled={submitting !== null} onClick={() => void submit('reject')} className="inline-flex h-8 items-center gap-1 rounded-md border border-border bg-surface px-2.5 text-xs font-semibold text-text-600 hover:bg-panel-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent disabled:opacity-50">
          <X className="h-3.5 w-3.5" />{submitting === 'reject' ? '提交中' : '拒绝'}
        </button>
        {item.proposedScope.object !== 'global' && (
          <button type="button" disabled={submitting !== null} onClick={() => void submit('adjust')} className="inline-flex h-8 items-center rounded-md border border-border bg-surface px-2.5 text-xs font-semibold text-text-700 hover:bg-panel-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent disabled:opacity-50">
            {submitting === 'adjust' ? '提交中' : '调整为全局'}
          </button>
        )}
        <button type="button" disabled={submitting !== null} onClick={() => void submit('approve')} className="inline-flex h-8 items-center gap-1 rounded-md bg-text-900 px-3 text-xs font-semibold text-white hover:bg-text-800 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent disabled:opacity-50">
          <Check className="h-3.5 w-3.5" />{submitting === 'approve' ? '提交中' : '批准'}
        </button>
      </div>
    </article>
  );
}
