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
    ? state.contentByProjectId[projectId]?.design.theme ?? state.projects.find(project => project.id === projectId)?.theme ?? ''
    : '');
  const setProjectTheme = useProjectStore(state => state.setProjectTheme);
  const [themes, setThemes] = useState<Theme[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [applying, setApplying] = useState(false);
  const loadingRef = useRef(false);
  const applyingRef = useRef(false);

  const load = useCallback(async () => {
    if (loadingRef.current) return;
    loadingRef.current = true;
    setLoading(true);
    setError(false);
    try {
      const response = await repositoriesApi.listThemes();
      setThemes(response.themes);
    } catch {
      setError(true);
    } finally {
      loadingRef.current = false;
      setLoading(false);
    }
  }, []);

  useEffect(() => { if (projectId) void load(); }, [projectId, load]);

  const apply = async (theme: Theme) => {
    if (!projectId || theme.id === themeId || applyingRef.current) return;
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

  const name = themes.find(theme => theme.id === themeId)?.name || themeId || '选择主题';
  return (
    <DropdownMenu onOpenChange={open => { if (open) void load(); }}>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          aria-label="切换主题"
          aria-busy={applying}
          disabled={!projectId || applying}
          title={applying ? '正在应用主题' : `切换主题：${name}`}
          className="ml-1.5 inline-flex h-8 shrink-0 items-center gap-1.5 rounded-full bg-panel-muted px-3 text-xs font-medium text-text-600 transition-colors hover:bg-accent-soft hover:text-accent focus-visible:bg-accent-soft data-[state=open]:bg-accent-soft data-[state=open]:text-accent disabled:cursor-not-allowed disabled:opacity-50"
        >
          {applying
            ? <Loader2 className="h-3.5 w-3.5 shrink-0 animate-spin motion-reduce:animate-none" />
            : <Palette className="h-3.5 w-3.5 shrink-0" />}
          <span className="max-w-24 truncate">{name}</span>
          <ChevronDown className="h-3 w-3 shrink-0" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-60">
        <DropdownMenuLabel className="text-[11px] font-normal text-text-500">主题选择</DropdownMenuLabel>
        {error ? (
          <DropdownMenuItem onSelect={event => { event.preventDefault(); void load(); }}>加载失败，点击重试</DropdownMenuItem>
        ) : themes.length === 0 ? (
          <DropdownMenuItem disabled>{loading ? '正在加载主题…' : '暂无可用主题'}</DropdownMenuItem>
        ) : themes.map(theme => (
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
