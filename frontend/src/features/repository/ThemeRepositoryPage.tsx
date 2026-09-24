import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { repositoriesApi } from '../../api/repositories';
import type { Theme, ThemeTag } from '../../api/types';
import { showGlobalError } from '../../stores/toastStore';
import { cn } from '../../lib/utils';
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
  SegmentedControl,
} from './RepositoryPrimitives';
import { RepositoryEditDialog } from './RepositoryEditDialog';
import { RepositoryShell } from './RepositoryShell';
import {
  themeShowcaseModes,
  type ThemeShowcaseMode,
} from './themeShowcase';
import { clearThemeExampleCache, ThemePreview, ThemePreviewGallery } from './ThemePreview';

const themeTagLabels: Record<ThemeTag, string> = {
  minimal: '极简',
  business: '商务',
  technology: '科技',
  cool: '清冷',
  warm: '温暖',
  other: '其它',
};

const themeTagOrder = Object.keys(themeTagLabels) as ThemeTag[];

export function ThemeRepositoryPage({ active = true }: { active?: boolean }) {
  const [themes, setThemes] = useState<Theme[]>([]);
  const [selectedId, setSelectedId] = useState('');
  const [query, setQuery] = useState('');
  const [filter, setFilter] = useState<ThemeTag | 'all'>('all');
  const [mode, setMode] = useState<ThemeShowcaseMode>('cover');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [editOpen, setEditOpen] = useState(false);
  const [pending, setPending] = useState<string | null>(null);
  const pendingRef = useRef(false);
  const loadedRef = useRef(false);
  const requestRef = useRef(0);
  const load = useCallback(async () => {
    const request = ++requestRef.current;
    if (!loadedRef.current) setLoading(true);
    setError('');
    try {
      const response = await repositoriesApi.listThemes();
      if (request !== requestRef.current) return;
      const details = response.themes;
      loadedRef.current = true;
      setThemes(current => JSON.stringify(current) === JSON.stringify(details) ? current : details);
      setSelectedId((value) => value || details[0]?.id || '');
    } catch (cause) {
      if (request !== requestRef.current) return;
      const message = cause instanceof Error ? cause.message : '主题仓库加载失败';
      if (loadedRef.current) showGlobalError(message);
      else setError(message);
    } finally {
      if (request === requestRef.current) setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (active) void load();
    else setEditOpen(false);
  }, [active, load]);
  const visible = useMemo(() => themes.filter((theme) =>
    (filter === 'all' || theme.tags.includes(filter)) &&
    `${theme.name} ${theme.description} ${theme.tags.flatMap((tag) => [tag, themeTagLabels[tag]]).join(' ')}`
      .toLowerCase().includes(query.toLowerCase())), [filter, query, themes]);
  const selected = visible.find((theme) => theme.id === selectedId) ?? visible[0];
  const toggle = async (theme: Theme) => {
    if (pendingRef.current) return;
    pendingRef.current = true;
    setPending(theme.id);
    const disabled = !theme.disabled;
    setThemes(current => current.map(value => value.id === theme.id ? { ...value, disabled } : value));
    try {
      const updated = await repositoriesApi.setThemeDisabled(theme.id, disabled);
      setThemes(current => current.map(value => value.id === theme.id ? { ...value, ...updated } : value));
    } catch (cause) {
      setThemes(current => current.map(value => value.id === theme.id ? { ...value, disabled: theme.disabled } : value));
      showGlobalError(cause instanceof Error ? cause.message : '主题状态更新失败');
    } finally {
      pendingRef.current = false;
      setPending(null);
    }
  };
  const deleteTheme = async (theme: Theme) => {
    try {
      await repositoriesApi.deleteTheme(theme.id);
      setThemes((current) => current.filter((value) => value.id !== theme.id));
      setSelectedId('');
    } catch (cause) {
      showGlobalError(cause instanceof Error ? cause.message : '主题删除失败');
      throw cause;
    }
  };
  const updateTheme = async (value: { name: string; description: string; tags: ThemeTag[] }) => {
    if (!selected) return;
    try {
      const updated = await repositoriesApi.updateTheme(selected.id, value);
      setThemes((current) => current.map((theme) => theme.id === updated.id ? { ...theme, ...updated } : theme));
    } catch (cause) {
      showGlobalError(cause instanceof Error ? cause.message : '主题更新失败');
      throw cause;
    }
  };

  return (
    <RepositoryShell section="theme" onRefresh={() => { clearThemeExampleCache(); void load(); }}>
      <div className="flex h-full min-h-0 flex-col">
        <RepositoryPageHeader title="主题" query={query} onQueryChange={setQuery} searchLabel="搜索主题" />
        {loading ? <RepositoryLoading aside /> : error ? <RepositoryState text={error} error /> : (
          <RepositoryWorkspace>
            <RepositoryCatalog
              label="主题列表"
              controls={(['all', ...themeTagOrder] as const).map((tag) => (
                <RepositoryFilterButton key={tag} active={filter === tag} onClick={() => setFilter(tag)}>
                  {tag === 'all' ? '全部' : themeTagLabels[tag]}
                </RepositoryFilterButton>
              ))}
            >
              {visible.length === 0
                ? <RepositoryState text="没有匹配的主题" className="min-h-40" />
                : themes.map((theme) => {
                  const active = selected?.id === theme.id;
                  return (
                    <div key={theme.id} hidden={!visible.includes(theme)}>
                      <RepositoryDirectoryItem
                        active={active}
                        disabled={theme.disabled}
                        name={theme.name}
                        description={theme.description}
                        visual={<ThemePreview key={theme.appearance?.hash ?? theme.style_hash} theme={theme} miniature />}
                        visualAspect="video"
                        visualClassName="rounded-sm"
                        onClick={() => setSelectedId(theme.id)}
                      />
                    </div>
                  );
                })}
            </RepositoryCatalog>
            {selected && (
              <RepositoryDetail
                active={active}
                label="主题详情"
                title={selected.name}
                description={selected.description}
                openUrl={selected.open_url}
                properties={<RepositoryTagList tags={selected.tags.map((tag) => themeTagLabels[tag])} />}
                actions={(
                  <div className="flex items-center gap-2.5">
                    <span className={cn('text-xs font-semibold', selected.disabled ? 'text-text-600' : 'text-success')}>
                      {selected.disabled ? '已关闭' : '已启用'}
                    </span>
                    <button type="button" role="switch" aria-checked={!selected.disabled} aria-label="切换主题状态"
                      disabled={pending !== null} onClick={() => void toggle(selected)}
                      className={cn('h-5 w-9 rounded-full p-0.5 transition-colors focus-visible:outline-none disabled:opacity-50', selected.disabled ? 'bg-border-strong' : 'bg-success')}>
                      <span className={cn('block h-4 w-4 rounded-full bg-white shadow-[0_1px_3px_rgba(23,32,43,0.25)] transition-transform', !selected.disabled && 'translate-x-4')} />
                    </button>
                  </div>
                )}
                contentClassName="flex flex-col p-5 md:p-7 xl:p-9"
                deleteNoun="主题"
                onEdit={() => setEditOpen(true)}
                onDelete={() => deleteTheme(selected)}
              >
                <div className="mb-5 flex shrink-0 justify-end">
                  <SegmentedControl value={mode} options={themeShowcaseModes} onChange={setMode} label="预览页面" />
                </div>
                <div className="flex min-h-0 flex-1 items-center justify-center">
                  <div className="aspect-video max-h-full w-full max-w-5xl overflow-hidden rounded-lg border border-border-strong">
                    <ThemePreviewGallery themes={themes} selected={selected} mode={mode} />
                  </div>
                </div>
              </RepositoryDetail>
            )}
            {!selected && <RepositoryState text="请选择一个主题" className="min-h-[420px] bg-canvas/70" />}
          </RepositoryWorkspace>
        )}
      </div>
      {selected && (
        <RepositoryEditDialog
          open={active && editOpen}
          title="编辑主题"
          value={{ name: selected.name, description: selected.description, tags: selected.tags }}
          tagOptions={themeTagOrder.map((tag) => ({ value: tag, label: themeTagLabels[tag] }))}
          onOpenChange={setEditOpen}
          onSave={updateTheme}
        />
      )}
    </RepositoryShell>
  );
}
