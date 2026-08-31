import type { CSSProperties } from 'react';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { repositoriesApi } from '../../api/repositories';
import type { Theme } from '../../api/types';
import { cn } from '../../lib/utils';
import { showGlobalError } from '../../stores/toastStore';
import {
  RepositoryCatalog,
  RepositoryDetail,
  RepositoryDirectoryItem,
  RepositoryLoading,
  RepositoryPageHeader,
  RepositoryState,
  RepositoryWorkspace,
  SegmentedControl,
} from './RepositoryPrimitives';
import { RepositoryShell } from './RepositoryShell';
import {
  buildThemeShowcase,
  themePalette,
  themeShowcaseModes,
  themeTypography,
  type ThemeShowcaseMode,
} from './themeShowcase';

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
  const [themes, setThemes] = useState<Theme[]>([]);
  const [selectedId, setSelectedId] = useState('');
  const [query, setQuery] = useState('');
  const [mode, setMode] = useState<ThemeShowcaseMode>('cover');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

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

  const visible = useMemo(() => themes.filter((theme) =>
    `${theme.name} ${theme.description}`.toLowerCase().includes(query.toLowerCase())), [query, themes]);
  const selected = visible.find((theme) => theme.id === selectedId) ?? visible[0];
  const colors = selected ? themePalette(selected) : [];
  const typography = selected ? themeTypography(selected) : null;

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

  return (
    <RepositoryShell section="theme" onRefresh={() => void load()}>
      <div className="flex h-full min-h-0 flex-col">
        <RepositoryPageHeader title="主题" query={query} onQueryChange={setQuery} searchLabel="搜索主题" />
        {loading ? <RepositoryLoading aside /> : error ? <RepositoryState text={error} error /> : (
          <RepositoryWorkspace>
            <RepositoryCatalog
              label="主题列表"
              controls={<span className="truncate text-xs font-semibold text-text-400">主题分类暂未定义</span>}
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
            {selected && typography && (
              <RepositoryDetail
                label="主题详情"
                title={selected.name}
                description={selected.description}
                openUrl={selected.open_url}
                properties={(
                  <>
                    <div className="flex items-center gap-1.5">
                      <span className="text-[11px] font-semibold text-text-400">色板</span>
                      <span className="flex gap-1" aria-label="主题色板">
                        {colors.map((color, index) => <i key={`${color}-${index}`} className="h-4 w-4 rounded-[4px] border border-black/10 shadow-[inset_0_1px_0_rgba(255,255,255,0.25)]" style={{ background: color }} title={color} />)}
                      </span>
                    </div>
                    <div className="flex min-w-0 items-center gap-1.5">
                      <span className="shrink-0 text-[11px] font-semibold text-text-400">字体</span>
                      <span className="truncate text-xs font-semibold text-text-900" style={{ fontFamily: typography.displayStack }}>{typography.label}</span>
                    </div>
                  </>
                )}
                actions={<SegmentedControl value={mode} options={themeShowcaseModes} onChange={setMode} label="预览页面" />}
                contentClassName="flex items-center justify-center p-5 md:p-7 xl:p-9"
                deleteNoun="主题"
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
    </RepositoryShell>
  );
}
