import { useCallback, useEffect, useMemo, useState } from 'react';
import { Pause } from 'lucide-react';
import { repositoriesApi } from '../../api/repositories';
import type { ComponentReference, ComponentTag, Theme } from '../../api/types';
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
import { RepositoryEditDialog } from './RepositoryEditDialog';
import { RepositoryShell } from './RepositoryShell';
import { IsolatedSlidePreview } from '../viewer/IsolatedSlidePreview';
import type { RuntimeSlide } from '../viewer/previewProtocol';

function ScaledComponentPreview({html,title,className,theme}: {html:string;title:string;className?:string;theme:Theme|null}) {
  const slides=useMemo<RuntimeSlide[]>(()=>theme?[{
    id:'component-preview',html:`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><style>.component-content{display:grid;place-items:center}.component-stage{width:100%;max-width:1200px}.component-stage>svg{width:100%}</style></head><body><main class="slide-stage"><section class="slide-content component-content"><div class="component-stage">${html}</div></section></main></body></html>`,
    frame:{slide_id:'component-preview',theme_id:theme.id,appearance:theme.appearance,canvas:{width:1920,height:1080,aspect_ratio:'16:9'},deck_title:'组件',ordinal:1,total:1,role:'content',section:{id:'components',title:'组件',index:1},numbering:{visible:false,format:'number'},chrome:[]},
  }]:[],[html,theme]);
  return <div className={cn('relative overflow-hidden',className)}>
    {theme && <IsolatedSlidePreview slides={slides} index={0} title={title} className="h-full w-full border-0" />}
  </div>;
}

const componentTagLabels: Record<ComponentTag, string> = {
  card: '卡片',
  chart: '统计图',
  table: '表格',
  list: '列表',
  process: '流程',
  metric: '指标',
  other: '其它',
};

function componentTagLabel(tag: ComponentTag): string {
  return componentTagLabels[tag];
}

const componentTagOrder = Object.keys(componentTagLabels) as ComponentTag[];

export function ComponentRepositoryPage() {
  const [theme, setTheme] = useState<Theme | null>(null);
  const [components, setComponents] = useState<ComponentReference[]>([]);
  const [selectedId, setSelectedId] = useState('');
  const [query, setQuery] = useState('');
  const [filter, setFilter] = useState<ComponentTag | 'all'>('all');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [pending, setPending] = useState<string | null>(null);
  const [editOpen, setEditOpen] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const [response, previewTheme] = await Promise.all([repositoriesApi.listComponents(), repositoriesApi.getTheme("editorial-serif")]);
      setTheme(previewTheme);
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

  const toggle = async (component: ComponentReference) => {
    const disabled = !component.disabled;
    setPending(component.id);
    setComponents((current) => current.map((value) => value.id === component.id ? { ...value, disabled } : value));
    try {
      const updated = await repositoriesApi.setComponentDisabled(component.id, disabled);
      setComponents((current) => current.map((value) => value.id === component.id ? { ...value, ...updated } : value));
    } catch (cause) {
      setComponents((current) => current.map((value) => value.id === component.id ? { ...value, disabled: component.disabled } : value));
      showGlobalError(cause instanceof Error ? cause.message : '组件状态更新失败');
    } finally {
      setPending(null);
    }
  };
  const updateComponent = async (value: { name: string; description: string; tags: ComponentTag[] }) => {
    if (!selected) return;
    try {
      const updated = await repositoriesApi.updateComponent(selected.id, value);
      setComponents((current) => current.map((component) => component.id === updated.id ? { ...component, ...updated } : component));
    } catch (cause) {
      showGlobalError(cause instanceof Error ? cause.message : '组件更新失败');
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
                      disabled={component.disabled}
                      name={component.name}
                      description={component.description}
                      visual={component.disabled ? (
                        <Pause className="h-4 w-4" strokeWidth={1.75} />
                      ) : (
                        <ScaledComponentPreview
                  theme={theme}
                          title={`${component.name} 缩略预览`}
                          html={component.html ?? ''}
                          className="h-full w-full"
                        />
                      )}
                      visualClassName={component.disabled ? 'h-9 w-9 rounded-full border-0 bg-panel-muted text-text-400' : undefined}
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
                actions={(
                  <div className="flex items-center gap-2.5">
                    <span className={cn('text-xs font-semibold', selected.disabled ? 'text-text-600' : 'text-success')}>
                      {selected.disabled ? '已关闭' : '已启用'}
                    </span>
                    <button
                      type="button"
                      role="switch"
                      aria-checked={!selected.disabled}
                      aria-label="切换组件状态"
                      disabled={pending === selected.id}
                      onClick={() => void toggle(selected)}
                      className={cn(
                        'h-5 w-9 rounded-full p-0.5 transition-colors focus-visible:outline-none disabled:opacity-50',
                        selected.disabled ? 'bg-border-strong' : 'bg-success',
                      )}
                    >
                      <span className={cn('block h-4 w-4 rounded-full bg-white shadow-[0_1px_3px_rgba(23,32,43,0.25)] transition-transform', !selected.disabled && 'translate-x-4')} />
                    </button>
                  </div>
                )}
                contentClassName="flex items-center justify-center overflow-hidden p-5 md:p-7"
                deleteNoun="组件"
                onEdit={() => setEditOpen(true)}
                onDelete={() => deleteComponent(selected)}
              >
                <ScaledComponentPreview
                  theme={theme}
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
      {selected && (
        <RepositoryEditDialog
          open={editOpen}
          title="编辑组件"
          value={{ name: selected.name, description: selected.description, tags: selected.tags }}
          tagOptions={componentTagOrder.map((tag) => ({ value: tag, label: componentTagLabels[tag] }))}
          onOpenChange={setEditOpen}
          onSave={updateComponent}
        />
      )}
    </RepositoryShell>
  );
}
