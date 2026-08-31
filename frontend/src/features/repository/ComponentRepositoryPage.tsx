import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { repositoriesApi } from '../../api/repositories';
import type { ComponentReference, ComponentTag } from '../../api/types';
import { cn } from '../../lib/utils';
import { showGlobalError } from '../../stores/toastStore';
import {
  RepositoryCatalog,
  RepositoryDetail,
  RepositoryDirectoryItem,
  RepositoryFilterButton,
  RepositoryLoading,
  RepositoryPageHeader,
  RepositoryState,
  RepositoryTagList,
  RepositoryWorkspace,
} from './RepositoryPrimitives';
import { RepositoryShell } from './RepositoryShell';

const COMPONENT_PREVIEW_WIDTH = 960;
const COMPONENT_PREVIEW_HEIGHT = 540;

function componentPreview(html: string): string {
  return `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><style>:root{--color-bg:#fff;--color-fg:#17202b;--color-primary:#2f67f6;--color-accent:#c84953;--color-muted:#e9edf2;--color-surface:#fff;--color-border:#d9e0e8;--font-sans:Aptos,Arial,sans-serif;--font-serif:Georgia,serif;--font-mono:"SFMono-Regular",Consolas,monospace;--text-h1:36px;--text-body:19px;--text-caption:13px;--space-2:8px;--space-3:12px;--space-4:16px;--space-6:24px;--radius-md:7px;--radius-lg:10px;--shadow-card:0 10px 28px rgba(51,65,85,.12)}*{box-sizing:border-box}html,body{margin:0;width:100%;height:100%;overflow:hidden}body{display:grid;place-items:center;padding:38px;font-family:var(--font-sans);color:var(--color-fg);background:linear-gradient(145deg,#f8fafc,#edf1f6)}.component-stage{display:grid;place-items:center;width:100%;max-width:880px}.component-stage>*:not(style):not(script){max-width:100%}.component-stage>figure,.component-stage>dl{width:100%;justify-self:stretch}</style></head><body><div class="component-stage">${html}</div></body></html>`;
}

function ScaledComponentPreview({
  html,
  title,
  className,
}: {
  html: string;
  title: string;
  className?: string;
}) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [scale, setScale] = useState(1);

  useLayoutEffect(() => {
    const container = containerRef.current;
    if (!container) return undefined;

    const measure = () => {
      const { width, height } = container.getBoundingClientRect();
      if (width <= 0 || height <= 0) return;
      const nextScale = Math.min(
        width / COMPONENT_PREVIEW_WIDTH,
        height / COMPONENT_PREVIEW_HEIGHT,
      );
      setScale((current) => Math.abs(current - nextScale) < 0.001 ? current : nextScale);
    };

    measure();
    if (typeof ResizeObserver === 'undefined') {
      window.addEventListener('resize', measure);
      return () => window.removeEventListener('resize', measure);
    }

    const observer = new ResizeObserver(measure);
    observer.observe(container);
    return () => observer.disconnect();
  }, []);

  return (
    <div ref={containerRef} className={cn('relative overflow-hidden', className)}>
      <iframe
        title={title}
        sandbox=""
        srcDoc={componentPreview(html)}
        width={COMPONENT_PREVIEW_WIDTH}
        height={COMPONENT_PREVIEW_HEIGHT}
        className="pointer-events-none absolute left-1/2 top-1/2 border-0 bg-white"
        style={{
          width: COMPONENT_PREVIEW_WIDTH,
          height: COMPONENT_PREVIEW_HEIGHT,
          marginLeft: -(COMPONENT_PREVIEW_WIDTH / 2),
          marginTop: -(COMPONENT_PREVIEW_HEIGHT / 2),
          transform: `scale(${scale})`,
          transformOrigin: 'center center',
        }}
      />
    </div>
  );
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

const componentTagOrder = Object.keys(componentTagLabels) as ComponentTag[];

export function ComponentRepositoryPage() {
  const [components, setComponents] = useState<ComponentReference[]>([]);
  const [selectedId, setSelectedId] = useState('');
  const [query, setQuery] = useState('');
  const [filter, setFilter] = useState<ComponentTag | 'all'>('all');
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

  const filters = useMemo(() => [
    'all' as const,
    ...componentTagOrder.filter((tag) => components.some((component) => component.tags.includes(tag))),
  ], [components]);
  const visible = useMemo(() => components.filter((component) =>
    (filter === 'all' || component.tags.includes(filter)) &&
    `${component.name} ${component.description} ${component.tags.flatMap((tag) => [tag, componentTagLabel(tag)]).join(' ')}`.toLowerCase().includes(query.toLowerCase())), [components, filter, query]);
  const selected = visible.find((component) => component.id === selectedId) ?? visible[0];

  const deleteComponent = async (component: ComponentReference) => {
    try {
      await repositoriesApi.deleteComponent(component.id);
      setComponents((current) => current.filter((value) => value.id !== component.id));
      setSelectedId('');
    } catch (cause) {
      showGlobalError(cause instanceof Error ? cause.message : '组件删除失败');
      throw cause;
    }
  };

  return (
    <RepositoryShell section="component" onRefresh={() => void load()}>
      <div className="flex h-full min-h-0 flex-col">
        <RepositoryPageHeader title="组件" query={query} onQueryChange={setQuery} searchLabel="搜索组件" />
        {loading ? <RepositoryLoading aside /> : error ? <RepositoryState text={error} error /> : (
          <RepositoryWorkspace>
            <RepositoryCatalog
              label="组件列表"
              controls={filters.map((value) => (
                <RepositoryFilterButton
                  key={value}
                  active={filter === value}
                  onClick={() => setFilter(value)}
                >
                  {value === 'all' ? '全部' : componentTagLabel(value)}
                </RepositoryFilterButton>
              ))}
            >
              {visible.length === 0
                ? <RepositoryState text="没有匹配的组件" className="min-h-40" />
                : visible.map((component) => {
                  const active = selected?.id === component.id;
                  return (
                    <RepositoryDirectoryItem
                      key={component.id}
                      active={active}
                      name={component.name}
                      description={component.description}
                      visual={(
                        <ScaledComponentPreview
                          title={`${component.name} 缩略预览`}
                          html={component.html ?? ''}
                          className="h-full w-full"
                        />
                      )}
                      onClick={() => setSelectedId(component.id)}
                    />
                  );
                })}
            </RepositoryCatalog>
            {selected && (
              <RepositoryDetail
                label="组件详情"
                title={selected.name}
                description={selected.description}
                openUrl={selected.open_url}
                properties={<RepositoryTagList tags={selected.tags.map(componentTagLabel)} />}
                contentClassName="flex items-center justify-center overflow-hidden p-5 md:p-7"
                deleteNoun="组件"
                onDelete={() => deleteComponent(selected)}
              >
                <ScaledComponentPreview
                  title={`${selected.name} 组件预览`}
                  html={selected.html ?? ''}
                  className="aspect-video max-h-full w-full max-w-5xl rounded-lg border border-border-strong bg-white shadow-[0_18px_44px_rgba(51,65,85,0.18)]"
                />
              </RepositoryDetail>
            )}
            {!selected && <RepositoryState text="请选择一个组件" className="min-h-[420px] bg-canvas/70" />}
          </RepositoryWorkspace>
        )}
      </div>
    </RepositoryShell>
  );
}
