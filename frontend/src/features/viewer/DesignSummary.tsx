import { useEffect, useState } from 'react';
import { repositoriesApi } from '../../api/repositories';
import type { Design } from '../../api/types';
import { chromeLabel } from './semanticLabels';

export function DesignSummary({ design }: { design: Design }) {
  return (
    <section className="rounded-xl border border-border bg-surface p-5 shadow-sm">
      <h3 className="font-semibold text-text-900">全局视觉规范</h3>
      <div className="mt-4"><DesignDetails design={design} compact /></div>
    </section>
  );
}

export function DesignDetails({ design, compact = false }: { design: Design; compact?: boolean }) {
  const [themeError, setThemeError] = useState(false);
  const [theme, setTheme] = useState<{ id: string; name: string }>();
  useEffect(() => {
    let active = true; setThemeError(false); setTheme(undefined);
    if (design.theme) {
      void repositoriesApi.getTheme(design.theme).then((value) => {
        if (active) {setTheme({ id: value.id, name: value.name });setThemeError(false);}
      }).catch(() => { if (active) {setTheme(undefined);setThemeError(true);} });
    }
    return () => { active = false; };
  }, [design.theme]);
  const themeName = !design.theme ? '尚未选择主题' : themeError ? '主题不可用，请从主题仓库选择现有主题' : theme?.id === design.theme ? theme.name : '正在加载主题…';
  const direction = design.direction.trim();
  const directionLabel = direction && direction !== '待确定' ? direction : '视觉方向待确定';

  return (
    <dl className={compact ? 'space-y-3' : 'space-y-6'}>
      <div className="flex gap-4">
        <dt className="w-16 shrink-0 text-xs leading-6 text-text-400">主题</dt>
        <dd className="min-w-0 break-words text-sm font-semibold leading-6 text-text-900">{themeName}</dd>
      </div>
      <div className="flex gap-4">
        <dt className="w-16 shrink-0 text-xs leading-6 text-text-400">视觉方向</dt>
        <dd className="min-w-0 whitespace-pre-wrap break-words text-sm leading-7 text-text-700">{directionLabel}</dd>
      </div>
      {(!compact || design.chrome.length > 0) && (
        <div className="flex gap-4">
          <dt className="w-16 shrink-0 text-xs leading-6 text-text-400">页面装饰</dt>
          <dd className={compact ? 'flex flex-wrap gap-2' : 'min-w-0 flex-1 space-y-4 text-sm leading-6'}>
            {design.chrome.map((item, index) => compact ? (
              <span key={index} className="inline-flex items-center rounded-full border border-border bg-panel-muted px-2.5 py-0.5 text-xs text-text-900">
                {chromeLabel(item)}
              </span>
            ) : (
              <div key={index}>
                <p className="font-medium text-text-900">{chromeLabel(item)}</p>
                <p className="mt-1 whitespace-pre-wrap break-words text-text-600">{item.style.trim() || '样式待确定'}</p>
              </div>
            ))}
            {design.chrome.length === 0 && <p className="text-text-400">尚未设置页面装饰</p>}
          </dd>
        </div>
      )}
    </dl>
  );
}
