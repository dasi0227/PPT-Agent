import { ExternalLink, Search } from 'lucide-react';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { projectsApi } from '../../api/projects';
import { repositoriesApi } from '../../api/repositories';
import type { Theme } from '../../api/types';
import { showGlobalError, showGlobalSuccess } from '../../stores/toastStore';
import { useProjectStore } from '../../stores/projectStore';
import { RepositoryShell } from './RepositoryShell';

type PreviewMode = 'overview' | 'cover' | 'data';

function cssVariable(css: string, name: string): string {
  return css.match(new RegExp(`${name}\\s*:\\s*([^;]+)`))?.[1]?.trim() ?? '';
}

function themePreview(theme: Theme, mode: PreviewMode): string {
  const css = theme.css ?? '';
  const body = mode === 'cover'
    ? '<main class="slide-stage"><section class="slide-content"><p class="kicker">产品复盘</p><h1 class="slide-title">让观点先于装饰</h1><p class="slide-subtitle">统一的视觉语言让每一页更清晰。</p></section></main>'
    : mode === 'data'
      ? '<main class="slide-stage"><section class="slide-content"><p class="kicker">核心指标</p><h1 class="slide-title">增长来自更短的创作链路</h1><div class="card"><div class="metric"><strong class="metric-value">36%</strong><span class="metric-label">效率提升</span></div><div class="metric"><strong class="metric-value">4.8x</strong><span class="metric-label">内容复用</span></div></div></section></main>'
      : '<main class="slide-stage"><section class="slide-content"><p class="kicker">季度复盘</p><h1 class="slide-title">把等待时间还给创作</h1><p class="slide-subtitle">主题统一文字、容器与数据表达。</p><div class="card"><h2>季度增长</h2><div class="metric"><strong class="metric-value">42%</strong><span class="metric-label">目标完成</span></div></div></section></main>';
  return `<!doctype html><html><head><style>${css}</style><style>html,body{margin:0;width:100%;height:100%;overflow:hidden}.slide-stage{box-sizing:border-box;width:100vw;height:100vh;overflow:hidden}.slide-content{box-sizing:border-box;height:100%;padding:7%;display:flex;flex-direction:column;justify-content:center;gap:clamp(8px,2.4vw,22px)}.card{padding:clamp(10px,3vw,28px);display:flex;gap:clamp(12px,4vw,36px);align-items:center;max-width:720px}.slide-title{margin:0;max-width:15ch;font-size:clamp(20px,4vw,var(--text-title))}.slide-subtitle{margin:0;max-width:38ch}.metric{display:flex;flex-direction:column;gap:6px}.metric-value{font-size:clamp(24px,5vw,44px)}</style></head><body>${body}</body></html>`;
}

export function ThemeRepositoryPage() {
  const [themes, setThemes] = useState<Theme[]>([]);
  const [selectedId, setSelectedId] = useState('');
  const [query, setQuery] = useState('');
  const [mode, setMode] = useState<PreviewMode>('overview');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const activeProjectId = useProjectStore((state) => state.activeProjectId);
  const currentTheme = useProjectStore((state) => activeProjectId
    ? state.contentByProjectId[activeProjectId]?.design.theme
    : undefined);

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const response = await repositoriesApi.listThemes();
      const details = await Promise.all(response.themes.map((theme) => repositoriesApi.getTheme(theme.id)));
      setThemes(details);
      setSelectedId((value) => value || currentTheme || details[0]?.id || '');
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '主题仓库加载失败');
    } finally {
      setLoading(false);
    }
  }, [currentTheme]);

  useEffect(() => { void load(); }, [load]);
  const visible = useMemo(() => themes.filter((theme) =>
    `${theme.name} ${theme.description} ${theme.tags.join(' ')}`.toLowerCase().includes(query.toLowerCase())), [query, themes]);
  const selected = visible.find((theme) => theme.id === selectedId) ?? visible[0];
  const colors = selected ? ['--color-bg', '--color-fg', '--color-primary', '--color-accent']
    .map((name) => cssVariable(selected.css ?? '', name)).filter(Boolean) : [];
  const font = selected ? cssVariable(selected.css ?? '', '--font-sans') : '';

  const selectTheme = async (theme: Theme) => {
    const previous = selectedId;
    setSelectedId(theme.id);
    if (!activeProjectId) return;
    try {
      const project = await projectsApi.setTheme(activeProjectId, theme.id);
      useProjectStore.setState((state) => ({
        projects: state.projects.map((item) => item.id === project.id ? project : item),
      }));
      await useProjectStore.getState().loadProjectContent(activeProjectId);
      showGlobalSuccess(`已应用主题：${theme.name}`);
    } catch (cause) {
      setSelectedId(previous);
      showGlobalError(cause instanceof Error ? cause.message : '主题切换失败');
    }
  };

  return (
    <RepositoryShell section="theme" title="主题" onRefresh={() => void load()}>
      <div className="flex h-full min-h-0 flex-col">
        <header className="flex h-14 shrink-0 items-center justify-between gap-3 border-b border-border bg-panel px-4">
          <h1 className="text-base font-semibold">主题</h1>
          <label className="flex h-8 w-full max-w-64 items-center gap-2 rounded-md border border-border bg-surface px-2 text-text-400">
            <Search className="h-4 w-4" /><input value={query} onChange={(event) => setQuery(event.target.value)} className="min-w-0 flex-1 bg-transparent text-sm text-text-900 outline-none" placeholder="搜索主题" aria-label="搜索主题" />
          </label>
        </header>
        {loading ? <RepositoryState text="正在加载主题" /> : error ? <RepositoryState text={error} error /> : visible.length === 0 ? <RepositoryState text="没有匹配的主题" /> : (
          <div className="grid min-h-0 flex-1 grid-cols-1 md:grid-cols-[280px_minmax(0,1fr)]">
            <aside className="overflow-y-auto border-b border-border bg-panel p-2 md:border-b-0 md:border-r">
              {visible.map((theme) => (
                <button key={theme.id} type="button" onClick={() => void selectTheme(theme)} className={`mb-1 w-full rounded-md px-3 py-2 text-left ${selected?.id === theme.id ? 'bg-accent-soft' : 'hover:bg-panel-muted'}`}>
                  <span className="block text-sm font-semibold">{theme.name}</span>
                  <span className="mt-0.5 block line-clamp-2 text-xs text-text-600">{theme.description}</span>
                </button>
              ))}
            </aside>
            {selected && <section className="flex min-h-0 flex-col bg-canvas">
              <header className="flex flex-wrap items-center gap-x-5 gap-y-2 border-b border-border bg-panel px-4 py-3">
                <div className="flex items-center gap-1">
                  <h2 className="text-sm font-semibold">{selected.name}</h2>
                  <a href={selected.open_url} title="查看文件" aria-label="查看文件"><ExternalLink className="h-3.5 w-3.5 text-text-500" /></a>
                </div>
                <span className="text-xs text-text-600">{selected.tags.join(' / ')}</span>
                <span className="flex items-center gap-1">{colors.map((color) => <i key={color} className="h-4 w-4 rounded-sm border border-border" style={{ background: color }} title={color} />)}</span>
                {font && <span className="min-w-0 truncate text-xs text-text-600">{font}</span>}
                <div className="ml-auto flex rounded-md border border-border bg-surface p-0.5">
                  {(['overview', 'cover', 'data'] as PreviewMode[]).map((value) => <button key={value} type="button" onClick={() => setMode(value)} className={`h-7 rounded px-2 text-xs ${mode === value ? 'bg-accent-soft text-accent' : 'text-text-600'}`}>{value === 'overview' ? '综合页' : value === 'cover' ? '封面' : '数据页'}</button>)}
                </div>
              </header>
              <div className="flex min-h-0 flex-1 items-center justify-center p-4">
                <iframe title={`${selected.name} 主题预览`} sandbox="" srcDoc={themePreview(selected, mode)} className="aspect-video max-h-full w-full max-w-5xl border border-border bg-white shadow-canvas" />
              </div>
            </section>}
          </div>
        )}
      </div>
    </RepositoryShell>
  );
}

function RepositoryState({ text, error = false }: { text: string; error?: boolean }) {
  return <div role={error ? 'alert' : 'status'} className={`grid min-h-0 flex-1 place-items-center text-sm ${error ? 'text-danger' : 'text-text-400'}`}>{text}</div>;
}
