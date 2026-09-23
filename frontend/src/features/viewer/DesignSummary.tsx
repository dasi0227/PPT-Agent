import { useEffect, useState } from 'react';
import { repositoriesApi } from '../../api/repositories';
import type { Design } from '../../api/types';
import { chromeLabel } from './semanticLabels';

export function DesignSummary({ design }: { design: Design }) {
  const [theme, setTheme] = useState<{ id: string; name: string }>();
  useEffect(() => {
    let active = true;
    if (design.theme) {
      void repositoriesApi.getTheme(design.theme).then((value) => {
        if (active) setTheme({ id: value.id, name: value.name });
      }).catch(() => { if (active) setTheme(undefined); });
    }
    return () => { active = false; };
  }, [design.theme]);
  const themeName = theme?.id === design.theme ? theme.name : '当前主题';
  const direction = design.direction.trim();
  const directionLabel = direction && direction !== '待确定' ? direction : '视觉方向待确定';

  return (
    <section className="rounded-xl border border-border bg-surface p-5 shadow-sm">
      <h3 className="font-semibold text-text-900">全局视觉规范</h3>
      <dl className="mt-4 space-y-3">
        <div className="flex gap-4">
          <dt className="w-16 shrink-0 text-xs leading-6 text-text-400">主题</dt>
          <dd className="text-sm font-semibold leading-6 text-text-900">{themeName}</dd>
        </div>
        <div className="flex gap-4">
          <dt className="w-16 shrink-0 text-xs leading-6 text-text-400">视觉方向</dt>
          <dd className="text-sm leading-6 text-text-600">{directionLabel}</dd>
        </div>
        {design.chrome.length > 0 && (
          <div className="flex gap-4">
            <dt className="w-16 shrink-0 text-xs leading-6 text-text-400">页面装饰</dt>
            <dd className="flex flex-wrap gap-2">
              {design.chrome.map((item, index) => (
                <span
                  key={index}
                  className="inline-flex items-center rounded-full border border-border bg-panel-muted px-2.5 py-0.5 text-xs text-text-900"
                >
                  {chromeLabel(item)}
                </span>
              ))}
            </dd>
          </div>
        )}
      </dl>
    </section>
  );
}
