import type { CSSProperties } from 'react';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { repositoriesApi } from '../../api/repositories';
import type { Theme } from '../../api/types';
import { cn } from '../../lib/utils';
import {
  RepositoryFileLink,
  RepositoryLoading,
  RepositoryPageHeader,
  RepositoryState,
  RepositoryToolbar,
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
        'relative aspect-video overflow-hidden rounded-md bg-[var(--mini-bg)] shadow-[inset_0_0_0_1px_rgba(23,32,43,0.12)]',
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

  return (
    <RepositoryShell section="theme" onRefresh={() => void load()}>
      <div className="flex h-full min-h-0 flex-col">
        <RepositoryPageHeader title="主题" query={query} onQueryChange={setQuery} searchLabel="搜索主题" />
        {loading ? <RepositoryLoading aside /> : error ? <RepositoryState text={error} error /> : visible.length === 0 ? <RepositoryState text="没有匹配的主题" /> : (
          <div className="grid min-h-0 flex-1 grid-cols-1 overflow-hidden lg:grid-cols-[320px_minmax(0,1fr)]">
            <aside className="max-h-72 overflow-y-auto border-b border-border bg-panel p-2.5 lg:max-h-none lg:border-b-0 lg:border-r" aria-label="主题列表">
              {visible.map((theme) => {
                const active = selected?.id === theme.id;
                const palette = themePalette(theme);
                return (
                  <button
                    key={theme.id}
                    type="button"
                    onClick={() => setSelectedId(theme.id)}
                    className={cn(
                      'group mb-1.5 grid w-full grid-cols-[104px_minmax(0,1fr)] gap-3 rounded-lg border p-2 text-left transition-all active:translate-y-px focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-inset',
                      active
                        ? 'border-accent/35 bg-accent-soft shadow-[0_2px_8px_rgba(47,103,246,0.08)]'
                        : 'border-transparent hover:border-border hover:bg-surface',
                    )}
                  >
                    <ThemeMiniature theme={theme} active={active} />
                    <span className="min-w-0 py-0.5">
                      <span className="block truncate text-[13px] font-bold text-text-900">{theme.name}</span>
                      <span className="mt-1 block line-clamp-2 text-[11px] leading-[1.45] text-text-600">{theme.description}</span>
                      <span className="mt-2 flex gap-1" aria-hidden="true">
                        {palette.map((color, index) => <i key={`${color}-${index}`} className="h-3 w-3 rounded-[3px] border border-black/10" style={{ background: color }} />)}
                      </span>
                    </span>
                  </button>
                );
              })}
            </aside>
            {selected && typography && (
              <section className="flex min-h-0 min-w-0 flex-col bg-canvas/70">
                <RepositoryToolbar>
                  <div className="flex w-full min-w-0 items-center justify-between gap-6 py-3">
                    <div className="flex min-w-0 flex-col gap-2">
                      <div className="flex min-w-0 items-end gap-1">
                        <div className="flex shrink-0 items-center gap-1">
                          <h2 className="text-base font-bold leading-7 tracking-[-0.01em] text-text-900">{selected.name}</h2>
                          <RepositoryFileLink href={selected.open_url} />
                        </div>
                        <p className="mb-0.5 ml-2 min-w-0 truncate text-xs font-medium leading-4 text-text-600" title={selected.description}>
                          {selected.description}
                        </p>
                      </div>
                      <div className="flex min-w-0 items-center gap-4">
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
                      </div>
                    </div>
                    <div className="shrink-0">
                      <SegmentedControl value={mode} options={themeShowcaseModes} onChange={setMode} label="预览页面" />
                    </div>
                  </div>
                </RepositoryToolbar>
                <div className="flex min-h-0 flex-1 items-center justify-center overflow-auto p-5 md:p-7 xl:p-9">
                  <iframe
                    title={`${selected.name} 主题预览`}
                    sandbox=""
                    srcDoc={buildThemeShowcase(selected, mode)}
                    className="aspect-video max-h-full w-full max-w-5xl rounded-lg border border-border-strong bg-white shadow-[0_18px_44px_rgba(51,65,85,0.18)]"
                  />
                </div>
              </section>
            )}
          </div>
        )}
      </div>
    </RepositoryShell>
  );
}
