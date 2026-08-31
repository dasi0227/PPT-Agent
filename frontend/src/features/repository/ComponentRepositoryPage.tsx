import { useCallback, useEffect, useMemo, useState } from 'react';
import { repositoriesApi } from '../../api/repositories';
import type { ComponentReference, ComponentTag } from '../../api/types';
import { cn } from '../../lib/utils';
import {
  RepositoryFileLink,
  RepositoryPageHeader,
  RepositoryState,
  RepositoryTagList,
  RepositoryToolbar,
} from './RepositoryPrimitives';
import { RepositoryShell } from './RepositoryShell';

function componentPreview(html: string, compact = false): string {
  const h1 = compact ? '20px' : '36px';
  const body = compact ? '12px' : '19px';
  const caption = compact ? '9px' : '13px';
  const padding = compact ? '10px' : '38px';
  const space2 = compact ? '4px' : '8px';
  const space4 = compact ? '8px' : '16px';
  const space6 = compact ? '12px' : '24px';
  return `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><style>:root{--color-bg:#fff;--color-fg:#17202b;--color-primary:#2f67f6;--color-accent:#c84953;--color-muted:#e9edf2;--color-surface:#fff;--color-border:#d9e0e8;--font-sans:Aptos,Arial,sans-serif;--font-serif:Georgia,serif;--font-mono:"SFMono-Regular",Consolas,monospace;--text-h1:${h1};--text-body:${body};--text-caption:${caption};--space-2:${space2};--space-4:${space4};--space-6:${space6};--radius-md:7px;--radius-lg:10px;--shadow-card:0 10px 28px rgba(51,65,85,.12)}*{box-sizing:border-box}html,body{margin:0;width:100%;height:100%;overflow:hidden}body{display:grid;place-items:center;padding:${padding};font-family:var(--font-sans);color:var(--color-fg);background:linear-gradient(145deg,#f8fafc,#edf1f6)}.component-stage{display:grid;place-items:center;width:100%;max-width:${compact ? '300px' : '700px'}}.component-stage>*:not(style):not(script){max-width:100%}.component-stage>figure,.component-stage>dl{width:100%;justify-self:stretch}</style></head><body><div class="component-stage">${html}</div></body></html>`;
}

const componentTagLabels: Record<ComponentTag, string> = {
  card: '卡片',
  metric: '指标',
  comparison: '对比',
  quote: '引用',
  list: '列表',
  chart: '图表',
  process: '流程',
  timeline: '时间线',
  other: '其他',
};

function componentTagLabel(tag: ComponentTag): string {
  return componentTagLabels[tag];
}

function filterLabel(value: string): string {
  if (value === 'all') return '全部';
  if (value === 'data') return '数据';
  if (value === 'content') return '内容';
  if (value === 'flow') return '流程';
  return value;
}

function ComponentLoading() {
  return (
    <div role="status" aria-label="正在加载组件" className="grid min-h-0 flex-1 grid-cols-1 bg-canvas/60 lg:grid-cols-[minmax(0,11fr)_minmax(0,9fr)]">
      <div className="grid content-start grid-cols-1 gap-2.5 border-r border-border p-3 sm:grid-cols-2 2xl:grid-cols-3">
        {Array.from({ length: 6 }).map((_, index) => (
          <div key={index} className="overflow-hidden rounded-lg border border-border bg-surface">
            <div className="aspect-video animate-pulse bg-border/60" />
            <div className="space-y-2 p-3"><div className="h-4 w-1/2 animate-pulse rounded bg-border/70" /><div className="h-3 w-4/5 animate-pulse rounded bg-border/60" /></div>
          </div>
        ))}
      </div>
      <div className="min-h-72 animate-pulse rounded-lg border border-border bg-panel" />
    </div>
  );
}

export function ComponentRepositoryPage() {
  const [components, setComponents] = useState<ComponentReference[]>([]);
  const [selectedId, setSelectedId] = useState('');
  const [query, setQuery] = useState('');
  const [filter, setFilter] = useState('all');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const response = await repositoriesApi.listComponents();
      const details = await Promise.all(response.components.map((component) => repositoriesApi.getComponent(component.id)));
      setComponents(details);
      setSelectedId((value) => value || details[0]?.id || '');
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '组件仓库加载失败');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { void load(); }, [load]);

  const filters = useMemo(() => ['all', ...new Set(components.map((component) => component.kind).filter(Boolean) as string[])], [components]);
  const visible = useMemo(() => components.filter((component) =>
    (filter === 'all' || component.kind === filter) &&
    `${component.name} ${component.description} ${component.tags.flatMap((tag) => [tag, componentTagLabel(tag)]).join(' ')}`.toLowerCase().includes(query.toLowerCase())), [components, filter, query]);
  const selected = visible.find((component) => component.id === selectedId) ?? visible[0];

  return (
    <RepositoryShell section="component" onRefresh={() => void load()}>
      <div className="flex h-full min-h-0 flex-col">
        <RepositoryPageHeader title="组件" query={query} onQueryChange={setQuery} searchLabel="搜索组件" />
        <div className="flex h-11 shrink-0 items-center gap-1 overflow-x-auto border-b border-border bg-panel px-4 md:px-5" aria-label="组件筛选">
          {filters.map((value) => (
            <button
              key={value}
              type="button"
              onClick={() => setFilter(value)}
              className={cn(
                'h-7 shrink-0 rounded-md px-3 text-xs font-semibold transition-all active:translate-y-px focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-1',
                filter === value
                  ? 'bg-surface text-accent shadow-[0_1px_3px_rgba(51,65,85,0.12)] ring-1 ring-border'
                  : 'text-text-600 hover:bg-panel-muted hover:text-text-900',
              )}
            >
              {filterLabel(value)}
            </button>
          ))}
        </div>
        {loading ? <ComponentLoading /> : error ? <RepositoryState text={error} error /> : visible.length === 0 ? <RepositoryState text="没有匹配的组件" /> : (
          <div className="grid min-h-0 flex-1 grid-cols-1 overflow-hidden lg:grid-cols-[minmax(0,11fr)_minmax(0,9fr)]">
            <section className="overflow-y-auto border-b border-border bg-canvas/60 p-3 lg:border-b-0 lg:border-r" aria-label="组件画廊">
              <div className="grid grid-cols-1 gap-2.5 sm:grid-cols-2 2xl:grid-cols-3">
                {visible.map((component) => {
                  const active = selected?.id === component.id;
                  return (
                    <button
                      key={component.id}
                      type="button"
                      onClick={() => setSelectedId(component.id)}
                      className={cn(
                        'group min-w-0 overflow-hidden rounded-lg border bg-surface text-left transition-all active:translate-y-px focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2',
                        active
                          ? 'border-accent shadow-[0_0_0_1px_rgba(47,103,246,0.8),0_10px_24px_rgba(47,103,246,0.12)]'
                          : 'border-border shadow-[0_3px_10px_rgba(71,85,105,0.06)] hover:-translate-y-px hover:border-border-strong hover:shadow-[0_8px_20px_rgba(71,85,105,0.12)]',
                      )}
                    >
                      <div className="aspect-video overflow-hidden border-b border-border bg-panel-muted">
                        <iframe title={`${component.name} 缩略预览`} sandbox="" srcDoc={componentPreview(component.html ?? '', true)} className="pointer-events-none h-full w-full border-0" />
                      </div>
                      <div className="px-2.5 py-2">
                        <span className="block truncate text-[13px] font-bold text-text-900">{component.name}</span>
                        <span className="mt-1 block truncate text-[11px] text-text-600">{component.description}</span>
                      </div>
                    </button>
                  );
                })}
              </div>
            </section>
            {selected && (
              <aside className="flex min-h-[380px] min-w-0 flex-col bg-panel lg:min-h-0" aria-label="组件详情">
                <RepositoryToolbar>
                  <div className="min-w-0">
                    <div className="flex items-center gap-1">
                      <h2 className="truncate text-sm font-bold text-text-900">{selected.name}</h2>
                      <RepositoryFileLink href={selected.open_url} />
                    </div>
                    <RepositoryTagList tags={selected.tags.map(componentTagLabel)} limit={4} />
                  </div>
                </RepositoryToolbar>
                <div className="flex min-h-0 flex-1 items-center justify-center overflow-hidden bg-canvas/70 p-5">
                  <iframe
                    title={`${selected.name} 组件预览`}
                    sandbox=""
                    srcDoc={componentPreview(selected.html ?? '')}
                    className="aspect-[4/3] max-h-full w-full rounded-lg border border-border-strong bg-white shadow-[0_14px_34px_rgba(51,65,85,0.16)]"
                  />
                </div>
              </aside>
            )}
          </div>
        )}
      </div>
    </RepositoryShell>
  );
}
