import { useCallback, useEffect, useMemo, useState } from 'react';
import { Component } from 'lucide-react';
import { repositoriesApi } from '../../api/repositories';
import type { ComponentReference, ComponentTag } from '../../api/types';
import { useComponentStore } from '../../stores/componentStore';
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
import { ComponentPreview } from './ComponentPreview';
import { RepositoryPreviewCache } from './RepositoryPreviewCache';

function previewKey(component: ComponentReference) {
  return JSON.stringify([component.id, component.html]);
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
  const previews = useMemo(() => components.filter(component => component.content_state === 'ready').map(component => ({
    key: previewKey(component),
    content: <ComponentPreview title={`${component.name} 组件预览`} html={component.html ?? ''} />,
  })), [components]);

  const deleteComponent = async (component: ComponentReference) => {
    try {
      await repositoriesApi.deleteComponent(component.id);
      useComponentStore.setState({ loaded: false });
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
      useComponentStore.setState({ loaded: false });
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
      useComponentStore.setState({ loaded: false });
      setComponents((current) => current.map((component) => component.id === updated.id ? { ...component, ...updated } : component));
    } catch (cause) {
      showGlobalError(cause instanceof Error ? cause.message : '组件更新失败');
      throw cause;
    }
  };

  return (
    <RepositoryShell section="component" onRefresh={() => { useComponentStore.setState({ loaded: false }); void load(); }}>
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
                : components.map((component) => {
                  const active = selected?.id === component.id;
                  return (
                    <div key={component.id} hidden={!visible.includes(component)}>
                      <RepositoryDirectoryItem
                        active={active}
                        disabled={component.disabled}
                        name={component.name}
                        description={component.description}
                        icon={Component}
                        onClick={() => setSelectedId(component.id)}
                      />
                    </div>
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
                {selected.content_state !== 'ready' ? (
                  <RepositoryState text={selected.content_error ?? '资源文件不可用'} error />
                ) : (<>
                <div className="aspect-video max-h-full w-full max-w-5xl overflow-hidden rounded-lg border border-border-strong bg-white">
                  <RepositoryPreviewCache entries={previews} activeKey={previewKey(selected)} />
                </div>
                </>)}
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
