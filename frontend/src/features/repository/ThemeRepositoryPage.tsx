import type { CSSProperties } from 'react';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Check, Loader2, Paintbrush } from 'lucide-react';
import { useLocation } from 'react-router-dom';
import { repositoriesApi } from '../../api/repositories';
import type { Theme, ThemeTag } from '../../api/types';
import { Button } from '../../components/ui/primitives';
import { cn } from '../../lib/utils';
import { useProjectStore } from '../../stores/projectStore';
import { showGlobalError, showGlobalSuccess } from '../../stores/toastStore';
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
  buildThemeShowcase,
  themePalette,
  themeShowcaseModes,
  themeTypography,
  type ThemeShowcaseMode,
} from './themeShowcase';

const themeTagLabels: Record<ThemeTag, string> = {
  minimal: '极简',
  business: '商务',
  technology: '科技',
  cool: '清冷',
  warm: '温暖',
  other: '其它',
};

const themeTagOrder = Object.keys(themeTagLabels) as ThemeTag[];

function projectIdFromReturnTo(returnTo: unknown): string | null {
  if (typeof returnTo !== 'string') return null;
  const match = returnTo.match(/^\/projects\/([^/?#]+)/);
  if (!match) return null;
  try {
    return decodeURIComponent(match[1]);
  } catch {
    return null;
  }
}

function ThemeMiniature({ theme, active }: { theme: Theme; active: boolean }) {
  const colors = themePalette(theme);
  const typography = themeTypography(theme);
  const [background = '#ffffff', foreground = '#17202b', primary = '#2f67f6', accent = '#c84953'] = colors;
  const style = {
    '--mini-bg': background,
    '--mini-fg': foreground,
    '--mini-primary': primary,
    '--mini-accent': accent,
    '--mini-font': typography.displayStack,
  } as CSSProperties;

  return (
    <div
      style={style}
      className={cn(
        'relative h-full w-full overflow-hidden rounded-md bg-[var(--mini-bg)] shadow-[inset_0_0_0_1px_rgba(23,32,43,0.12)]',
        active && 'shadow-[inset_0_0_0_1px_rgba(47,103,246,0.28)]',
      )}
      aria-hidden="true"
    >
      <span className="absolute left-[11%] top-[18%] text-[18px] font-extrabold leading-none tracking-[-0.06em] text-[var(--mini-primary)]" style={{ fontFamily: 'var(--mini-font)' }}>Dasi</span>
      <span className="absolute right-[11%] top-[17%] h-[8px] w-[8px] bg-[var(--mini-accent)]" />
      <span className="absolute bottom-[25%] left-[11%] h-[4%] w-[62%] bg-[var(--mini-fg)] opacity-80" />
      <span className="absolute bottom-[14%] left-[11%] h-[3%] w-[42%] bg-[var(--mini-fg)] opacity-35" />
      <span className="absolute bottom-0 left-0 h-[5%] w-full bg-[var(--mini-primary)]" />
    </div>
  );
}

export function ThemeRepositoryPage() {
  const location = useLocation();
  const activeProjectId = useProjectStore((state) => state.activeProjectId);
  const projects = useProjectStore((state) => state.projects);
  const contentByProjectId = useProjectStore((state) => state.contentByProjectId);
  const loadingProjects = useProjectStore((state) => state.loadingProjects);
  const loadProjects = useProjectStore((state) => state.loadProjects);
  const setProjectTheme = useProjectStore((state) => state.setProjectTheme);
  const [themes, setThemes] = useState<Theme[]>([]);
  const [selectedId, setSelectedId] = useState('');
  const [query, setQuery] = useState('');
  const [filter, setFilter] = useState<ThemeTag | 'all'>('all');
  const [mode, setMode] = useState<ThemeShowcaseMode>('cover');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [applyingThemeId, setApplyingThemeId] = useState('');
  const [editOpen, setEditOpen] = useState(false);
  const applyingRef = useRef(false);
  const requestedProjectIds = useRef(new Set<string>());
  const projectId = activeProjectId ?? projectIdFromReturnTo(location.state?.returnTo);
  const currentProject = projects.find((project) => project.id === projectId);
  const currentThemeId = projectId
    ? contentByProjectId[projectId]?.design.theme ?? currentProject?.theme ?? ''
    : '';
  const projectReady = Boolean(projectId && (currentProject || contentByProjectId[projectId]));

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const response = await repositoriesApi.listThemes();
      const details = await Promise.all(response.themes.map((theme) => repositoriesApi.getTheme(theme.id)));
      setThemes(details);
      setSelectedId((value) => value || details[0]?.id || '');
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '主题仓库加载失败');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { void load(); }, [load]);
  useEffect(() => {
    if (!projectId || currentProject || requestedProjectIds.current.has(projectId)) return;
    requestedProjectIds.current.add(projectId);
    void loadProjects();
  }, [currentProject, loadProjects, projectId]);

  const visible = useMemo(() => themes.filter((theme) =>
    (filter === 'all' || theme.tags.includes(filter)) &&
    `${theme.name} ${theme.description} ${theme.tags.flatMap((tag) => [tag, themeTagLabels[tag]]).join(' ')}`
      .toLowerCase().includes(query.toLowerCase())), [filter, query, themes]);
  const selected = visible.find((theme) => theme.id === selectedId) ?? visible[0];
  const selectedIsCurrent = Boolean(selected && currentThemeId === selected.id);

  const applySelectedTheme = async () => {
    if (!projectId || !selected || selectedIsCurrent || applyingRef.current) return;
    applyingRef.current = true;
    setApplyingThemeId(selected.id);
    try {
      await setProjectTheme(projectId, selected.id);
      showGlobalSuccess(`已应用「${selected.name}」主题`);
    } catch (cause) {
      showGlobalError(cause instanceof Error ? cause.message : '主题应用失败');
    } finally {
      applyingRef.current = false;
      setApplyingThemeId('');
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
    <RepositoryShell section="theme" onRefresh={() => void load()}>
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
                : visible.map((theme) => {
                  const active = selected?.id === theme.id;
                  return (
                    <RepositoryDirectoryItem
                      key={theme.id}
                      active={active}
                      name={theme.name}
                      description={theme.description}
                      visual={<ThemeMiniature theme={theme} active={active} />}
                      onClick={() => setSelectedId(theme.id)}
                    />
                  );
                })}
            </RepositoryCatalog>
            {selected && (
              <RepositoryDetail
                label="主题详情"
                title={selected.name}
                description={selected.description}
                openUrl={selected.open_url}
                properties={<RepositoryTagList tags={selected.tags.map((tag) => themeTagLabels[tag])} />}
                actions={(
                  <div className="flex items-center gap-2">
                    <SegmentedControl value={mode} options={themeShowcaseModes} onChange={setMode} label="预览页面" />
                    <Button
                      type="button"
                      variant={selectedIsCurrent ? 'secondary' : 'primary'}
                      onClick={() => void applySelectedTheme()}
                      disabled={!projectReady || selectedIsCurrent || Boolean(applyingThemeId)}
                      aria-live="polite"
                      title={!projectId ? '请先打开一个项目' : undefined}
                      className={cn(
                        'h-7 px-2.5 text-xs focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-1',
                        selectedIsCurrent && 'disabled:border-accent/20 disabled:bg-accent-soft disabled:text-accent disabled:opacity-100',
                        applyingThemeId && 'disabled:opacity-70',
                      )}
                    >
                      {applyingThemeId
                        ? <Loader2 className="h-3.5 w-3.5 animate-spin motion-reduce:animate-none" strokeWidth={1.75} />
                        : selectedIsCurrent
                          ? <Check className="h-3.5 w-3.5" strokeWidth={1.9} />
                          : <Paintbrush className="h-3.5 w-3.5" strokeWidth={1.75} />}
                      {applyingThemeId
                        ? '应用中'
                        : selectedIsCurrent
                          ? '当前主题'
                            : projectId && loadingProjects && !projectReady
                            ? '读取项目'
                            : projectReady
                              ? '应用主题'
                              : projectId
                                ? '项目不可用'
                                : '无当前项目'}
                    </Button>
                  </div>
                )}
                contentClassName="flex items-center justify-center p-5 md:p-7 xl:p-9"
                deleteNoun="主题"
                onEdit={() => setEditOpen(true)}
                onDelete={() => deleteTheme(selected)}
              >
                <iframe
                  title={`${selected.name} 主题预览`}
                  sandbox=""
                  srcDoc={buildThemeShowcase(selected, mode)}
                  className="aspect-video max-h-full w-full max-w-5xl rounded-lg border border-border-strong bg-white shadow-[0_18px_44px_rgba(51,65,85,0.18)]"
                />
              </RepositoryDetail>
            )}
            {!selected && <RepositoryState text="请选择一个主题" className="min-h-[420px] bg-canvas/70" />}
          </RepositoryWorkspace>
        )}
      </div>
      {selected && (
        <RepositoryEditDialog
          open={editOpen}
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
