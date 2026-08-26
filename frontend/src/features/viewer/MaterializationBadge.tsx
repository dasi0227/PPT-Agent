import type { MaterializationState } from '../../api/types';
import { cn } from '../../lib/utils';

const labels: Record<MaterializationState, string> = {
  pending: '等待生成',
  not_materialized: '未生成',
  fresh: '已同步',
  spec_stale: '设计稿有更新',
  design_stale: '风格有更新',
  frame_stale: '页码已更新',
  unknown: '状态未知',
};

export function MaterializationBadge({ state }: { state: MaterializationState }) {
  return (
    <span className={cn(
      'rounded-full border px-2 py-0.5 text-[10px] font-medium',
      state === 'fresh' ? 'border-success/20 bg-success-soft text-success' :
        state === 'not_materialized' ? 'border-border bg-panel-muted text-text-600' :
          state === 'unknown' ? 'border-border bg-surface text-text-400' :
            'border-warning/20 bg-warning-soft text-warning',
    )}>
      {labels[state]}
    </span>
  );
}
