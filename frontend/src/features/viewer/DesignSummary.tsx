import type { Design } from '../../api/types';
import { chromeLabel, densityLabel } from './semanticLabels';

export function DesignSummary({ design, compact = false }: { design: Design; compact?: boolean }) {
  if (compact) {
    return (
      <section className="border-b border-border pb-3">
        <details className="group">
          <summary className="flex cursor-pointer list-none items-center gap-3 text-xs text-text-600 marker:content-none">
            <span className="font-semibold text-text-900">全局视觉规范</span>
            <span>{design.theme} · {densityLabel(design.density)}</span>
            <span className="ml-auto text-text-400 group-open:hidden">查看详情</span>
            <span className="ml-auto hidden text-text-400 group-open:inline">收起详情</span>
          </summary>
          <div className="mt-3 max-w-3xl text-sm text-text-600">
            <p>{design.direction}</p>
            {design.chrome.length > 0 && (
              <p className="mt-1 text-xs text-text-400">
                页面装饰：{design.chrome.map(chromeLabel).join(' · ')}
              </p>
            )}
          </div>
        </details>
      </section>
    );
  }

  return (
    <section className="rounded-xl border border-border bg-surface p-5 shadow-sm">
      <div className="flex items-center justify-between">
        <h3 className="font-semibold text-text-900">全局视觉规范</h3>
        <span className="text-xs text-text-400">rev {design.revision}</span>
      </div>
      <p className="mt-3 text-xs font-medium uppercase tracking-wide text-text-400">
        {design.theme} · {densityLabel(design.density)}
      </p>
      <p className="mt-2 text-sm text-text-600">{design.direction}</p>
      {design.chrome.length > 0 && (
        <p className="mt-2 text-xs text-text-400">
          页面装饰：{design.chrome.map(chromeLabel).join(' · ')}
        </p>
      )}
    </section>
  );
}
