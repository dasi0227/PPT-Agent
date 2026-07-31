import type { MaterializationState } from '../../api/types';
import { cn } from '../../lib/utils';

const labels: Record<MaterializationState, string> = {
  not_materialized: '未生成',
  fresh: '已同步',
  blueprint_stale: '蓝图有更新',
  design_stale: '风格有更新',
  unknown: '状态未知',
};

export function MaterializationBadge({ state }: { state: MaterializationState }) {
  return (
    <span className={cn(
      'rounded-full border px-2 py-0.5 text-[10px] font-medium',
      state === 'fresh' ? 'border-emerald-200 bg-emerald-50 text-emerald-700' :
        state === 'not_materialized' ? 'border-slate-200 bg-slate-50 text-slate-600' :
          state === 'unknown' ? 'border-slate-200 bg-white text-slate-500' :
            'border-amber-200 bg-amber-50 text-amber-700',
    )}>
      {labels[state]}
    </span>
  );
}
