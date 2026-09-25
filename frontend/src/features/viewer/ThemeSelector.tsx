import { useAppShortcuts } from '../../lib/useAppShortcuts';
import { useCallback, useEffect, useRef, useState } from 'react';
import { Check, ChevronDown, Loader2, Palette } from 'lucide-react';
import { repositoriesApi } from '../../api/repositories';
import type { Theme } from '../../api/types';
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuTrigger,
} from '../../components/ui/dropdown-menu';
import { cn } from '../../lib/utils';
import { useProjectStore } from '../../stores/projectStore';
import { showGlobalError } from '../../stores/toastStore';

export function ThemeSelector({ projectId }: { projectId: string | null }) {
  const themeId = useProjectStore(state => projectId
    ? state.contentByProjectId[projectId]?.theme ?? state.projects.find(project => project.id === projectId)?.theme ?? ''
    : '');
  const setProjectTheme = useProjectStore(state => state.setProjectTheme);
  const [open, setOpen] = useState(false);
  useEffect(() => { setOpen(false); }, [projectId]);
  const [themes, setThemes] = useState<Theme[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [applying, setApplying] = useState(false);
  const loadingRef = useRef<Promise<Theme[] | null> | null>(null);
  const applyingRef = useRef(false);
  const cyclingRef = useRef(false);

  const load = useCallback(() => {
    if (loadingRef.current) return loadingRef.current;
    setLoading(true);
    setError(false);
    loadingRef.current = repositoriesApi.listThemes().then(response => {
      setThemes(response.themes);
      return response.themes;
    }).catch(() => {
      setError(true);
      return null;
    }).finally(() => {
      loadingRef.current = null;
      setLoading(false);
    });
    return loadingRef.current;
  }, []);

  useEffect(() => { if (projectId) void load(); }, [projectId, load]);

  const apply = async (theme: Theme) => {
    if (!projectId || theme.disabled || theme.content_state !== 'ready' || theme.id === themeId || applyingRef.current) return;
    applyingRef.current = true;
    setApplying(true);
    try {
      await setProjectTheme(projectId, theme.id);
    } catch (cause) {
      showGlobalError(cause instanceof Error ? cause.message : '主题应用失败');
    } finally {
      applyingRef.current = false;
      setApplying(false);
    }
  };

  const cycleTheme = async () => {
    if (applyingRef.current || cyclingRef.current) return;
    cyclingRef.current = true;
    try {
      const current = await load();
      if (!current) { showGlobalError('主题加载失败'); return; }
      const available = current.filter(theme => !theme.disabled && theme.content_state === 'ready');
      if (!available.length) return;
      const nextIndex = (available.findIndex(theme => theme.id === themeId) + 1) % available.length;
      await apply(available[nextIndex]);
    } finally {
      cyclingRef.current = false;
    }
  };

  useAppShortcuts(projectId ? { 'deck.theme': () => { void cycleTheme(); } } : {});

  const name = themes.find(theme => theme.id === themeId)?.name || themeId || '选择主题';
  const enabledThemes = themes.filter(theme => !theme.disabled && theme.content_state === 'ready');
  return (
    <DropdownMenu open={open} onOpenChange={value => { setOpen(value); if (value) void load(); }}>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          aria-label="切换主题"
          aria-busy={applying}
          disabled={!projectId || applying}
          title={applying ? '正在应用主题' : `切换主题：${name}`}
          className="preview-theme-trigger flex h-8 min-w-0 items-center gap-1.5 whitespace-nowrap rounded-md bg-transparent px-2 text-left text-xs font-medium text-text-600 transition-colors hover:bg-accent-soft hover:text-accent focus-visible:bg-accent-soft focus-visible:outline-none data-[state=open]:bg-accent-soft data-[state=open]:text-accent disabled:cursor-not-allowed disabled:opacity-50"
        >
          {applying
            ? <Loader2 className="h-3.5 w-3.5 shrink-0 animate-spin motion-reduce:animate-none" />
            : <Palette className="h-3.5 w-3.5 shrink-0" />}
          <span className="shrink-0">主题</span>
          <span className="preview-theme-name min-w-0 flex-1 truncate">{name}</span>
          <ChevronDown className="h-3 w-3 shrink-0" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-[var(--radix-dropdown-menu-trigger-width)] min-w-52">
        <DropdownMenuLabel className="text-[11px] font-normal text-text-500">主题选择</DropdownMenuLabel>
        {error ? (
          <DropdownMenuItem onSelect={event => { event.preventDefault(); void load(); }}>加载失败，点击重试</DropdownMenuItem>
        ) : loading ? (
          <DropdownMenuItem disabled>正在加载主题…</DropdownMenuItem>
        ) : enabledThemes.length === 0 ? (
          <DropdownMenuItem disabled>暂无可用主题</DropdownMenuItem>
        ) : enabledThemes.map(theme => (
          <DropdownMenuItem
            key={theme.id}
            role="menuitemradio"
            aria-checked={theme.id === themeId}
            disabled={applying}
            onSelect={() => void apply(theme)}
            className={cn('gap-3', theme.id === themeId && 'bg-accent-soft text-accent')}
          >
            <span className="min-w-0 flex-1 whitespace-normal break-words">{theme.name}</span>
            {theme.id === themeId && <Check className="h-3.5 w-3.5 shrink-0" />}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
