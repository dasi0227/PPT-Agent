import { ExternalLink, Search } from 'lucide-react';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { repositoriesApi } from '../../api/repositories';
import type { ComponentReference } from '../../api/types';
import { RepositoryShell } from './RepositoryShell';

function componentPreview(html: string): string {
  return `<!doctype html><html><head><style>:root{--color-bg:#fff;--color-fg:#17202b;--color-primary:#2f67f6;--color-accent:#c84953;--color-muted:#e9edf2;--font-sans:Aptos,Arial,sans-serif;--font-serif:Georgia,serif;--text-h1:30px;--text-body:18px;--text-caption:13px;--space-2:8px;--space-4:16px;--space-6:24px;--radius-md:6px;--radius-lg:8px;--shadow-card:0 8px 24px rgba(51,65,85,.14)}html,body{margin:0;width:100%;height:100%;font-family:var(--font-sans);color:var(--color-fg);background:#f7f9fc}body{display:grid;place-items:center;padding:24px;box-sizing:border-box}body>*{max-width:100%}</style></head><body>${html}</body></html>`;
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
    `${component.name} ${component.description} ${component.tags.join(' ')}`.toLowerCase().includes(query.toLowerCase())), [components, filter, query]);
  const selected = visible.find((component) => component.id === selectedId) ?? visible[0];

  return (
    <RepositoryShell section="component" title="组件" onRefresh={() => void load()}>
      <div className="flex h-full min-h-0 flex-col">
        <header className="flex h-14 shrink-0 items-center justify-between gap-3 border-b border-border bg-panel px-4">
          <h1 className="text-base font-semibold">组件</h1>
          <label className="flex h-8 w-full max-w-72 items-center gap-2 rounded-md border border-border bg-surface px-2 text-text-400">
            <Search className="h-4 w-4" /><input value={query} onChange={(event) => setQuery(event.target.value)} className="min-w-0 flex-1 bg-transparent text-sm text-text-900 outline-none" placeholder="搜索名称、用途或标签" aria-label="搜索组件" />
          </label>
        </header>
        <div className="flex h-10 shrink-0 items-center gap-1 overflow-x-auto border-b border-border bg-panel px-4" aria-label="组件筛选">
          {filters.map((value) => <button key={value} type="button" onClick={() => setFilter(value)} className={`h-7 shrink-0 rounded-md px-3 text-xs font-semibold ${filter === value ? 'bg-surface text-accent shadow-sm ring-1 ring-border' : 'text-text-600 hover:bg-panel-muted'}`}>{value === 'all' ? '全部' : value}</button>)}
        </div>
        {loading ? <RepositoryState text="正在加载组件" /> : error ? <RepositoryState text={error} error /> : visible.length === 0 ? <RepositoryState text="没有匹配的组件" /> : (
          <div className="grid min-h-0 flex-1 grid-cols-1 lg:grid-cols-[minmax(360px,1fr)_minmax(420px,1.1fr)]">
            <section className="overflow-y-auto border-b border-border p-3 lg:border-b-0 lg:border-r">
              <div className="grid grid-cols-1 gap-3 xl:grid-cols-2">
                {visible.map((component) => (
                  <button key={component.id} type="button" onClick={() => setSelectedId(component.id)} className={`min-w-0 overflow-hidden rounded-md border bg-panel text-left ${selected?.id === component.id ? 'border-accent ring-1 ring-accent' : 'border-border hover:border-border-strong'}`}>
                    <div className="aspect-[16/8] bg-surface">
                      <iframe title={`${component.name} 缩略预览`} sandbox="" srcDoc={componentPreview(component.html ?? '')} className="pointer-events-none h-full w-full border-0" />
                    </div>
                    <div className="border-t border-border p-2.5">
                      <span className="block truncate text-sm font-semibold">{component.name}</span>
                      <span className="mt-0.5 block line-clamp-2 text-xs text-text-600">{component.description}</span>
                    </div>
                  </button>
                ))}
              </div>
            </section>
            {selected && <aside className="flex min-h-0 flex-col bg-canvas">
              <header className="flex shrink-0 flex-wrap items-center gap-2 border-b border-border bg-panel px-4 py-3">
                <h2 className="text-sm font-semibold">{selected.name}</h2>
                <a href={selected.open_url} title="查看文件" aria-label="查看文件"><ExternalLink className="h-3.5 w-3.5 text-text-500" /></a>
                <span className="text-xs text-text-600">{selected.tags.join(' / ')}</span>
              </header>
              <div className="min-h-[280px] flex-1 p-4">
                <iframe title={`${selected.name} 组件预览`} sandbox="" srcDoc={componentPreview(selected.html ?? '')} className="h-full min-h-[260px] w-full rounded-md border border-border bg-white shadow-canvas" />
              </div>
            </aside>}
          </div>
        )}
      </div>
    </RepositoryShell>
  );
}

function RepositoryState({ text, error = false }: { text: string; error?: boolean }) {
  return <div role={error ? 'alert' : 'status'} className={`grid min-h-0 flex-1 place-items-center text-sm ${error ? 'text-danger' : 'text-text-400'}`}>{text}</div>;
}
