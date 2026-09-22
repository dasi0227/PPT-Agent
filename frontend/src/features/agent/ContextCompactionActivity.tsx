import type { ContextCompactionTimelineItem } from './eventReducer';
import { CommandActivity } from './CommandActivity';
export function ContextCompactionActivity({ item }: { item: ContextCompactionTimelineItem }) {
  const before = item.maxTokens > 0 ? Math.round((item.beforeTokens / item.maxTokens) * 100) : 0;
  const after = item.maxTokens > 0 ? Math.round((item.afterTokens / item.maxTokens) * 100) : 0;
  return (
    <CommandActivity
      kind="compact"
      title={item.title}
      timestamp={item.timestamp}
      content={item.content}
      metadata={
        <>
          <span>{item.trigger === 'auto' ? '自动' : '手动'}</span>
          <span aria-hidden="true">·</span>
          <span>
            窗口 {before}% → <span className="text-success">{after}%</span>
          </span>
          <span aria-hidden="true">·</span>
          <span>
            回收{' '}
            <span className="text-danger">
              {(Math.max(0, item.reclaimedTokens) / 1000).toFixed(1)}k
            </span>{' '}
            Token
          </span>
        </>
      }
    />
  );
}
