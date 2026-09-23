import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { repositoriesApi } from '../../api/repositories';
import type { Theme, ThemeTag } from '../../api/types';
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
                actions={<SegmentedControl value={mode} options={themeShowcaseModes} onChange={setMode} label="预览页面" />}
                contentClassName="flex items-center justify-center p-5 md:p-7 xl:p-9"
                deleteNoun="主题"
                onEdit={() => setEditOpen(true)}
                onDelete={() => deleteTheme(selected)}
              >
                <div className="aspect-video max-h-full w-full max-w-5xl overflow-hidden rounded-lg border border-border-strong">
                  <ThemePreviewGallery themes={themes} selected={selected} mode={mode} />
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
