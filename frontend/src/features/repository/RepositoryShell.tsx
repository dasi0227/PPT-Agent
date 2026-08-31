import type { ReactNode } from 'react';
import { ArrowLeft, BookOpenText, Component, Palette, RefreshCw } from 'lucide-react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import { cn } from '../../lib/utils';
import { useProjectStore } from '../../stores/projectStore';
import { homeRoute, projectRoute, repositoryRoute, type RepositorySection } from '../workspace/routes';

const sections: Array<{
  id: RepositorySection;
  label: string;
  icon: typeof Palette;
}> = [
  { id: 'theme', label: '主题', icon: Palette },
  { id: 'component', label: '组件', icon: Component },
  { id: 'skill', label: '技能', icon: BookOpenText },
];

export function RepositoryShell({
  section,
  onRefresh,
  children,
}: {
  section: RepositorySection;
  onRefresh: () => void;
  children: ReactNode;
}) {
  const navigate = useNavigate();
  const location = useLocation();
  const activeProjectId = useProjectStore((state) => state.activeProjectId);
  const returnTo = typeof location.state?.returnTo === 'string'
    ? location.state.returnTo
    : activeProjectId
      ? projectRoute(activeProjectId)
      : homeRoute;
  const repositoryLocationState = { returnTo };
  return (
    <main className="flex h-[100dvh] max-h-[100dvh] min-h-0 min-w-0 flex-col overflow-hidden bg-workspace text-text-900">
      <header className="flex h-12 shrink-0 items-center border-b border-border-strong bg-surface px-3 shadow-[0_1px_0_rgba(255,255,255,0.75)]">
        <button
          type="button"
          onClick={() => navigate(returnTo)}
          className="flex min-w-0 items-center gap-2 rounded-md pr-2 font-bold transition-colors hover:bg-panel-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
          aria-label="返回项目"
        >
          <img src="/logo.jpg" alt="" className="h-8 w-8 rounded-md object-cover" />
          <span className="hidden truncate text-[15px] tracking-[-0.01em] sm:inline">Dasi PPT Agent</span>
        </button>
        <span className="mx-2 h-5 w-px bg-border" aria-hidden="true" />
        <span className="min-w-0 flex-1 truncate text-sm font-semibold text-text-600">仓库</span>
        <button type="button" onClick={onRefresh} className="grid h-8 w-8 place-items-center rounded-md text-text-600 transition-colors hover:bg-panel-muted hover:text-text-900 active:translate-y-px" title="刷新仓库" aria-label="刷新仓库">
          <RefreshCw className="h-4 w-4" strokeWidth={1.75} />
        </button>
        <Link to={returnTo} className="ml-1 grid h-8 w-8 place-items-center rounded-md text-text-600 transition-colors hover:bg-panel-muted hover:text-text-900 active:translate-y-px" title="返回项目" aria-label="返回项目">
          <ArrowLeft className="h-4 w-4" strokeWidth={1.75} />
        </Link>
      </header>
      <div className="flex min-h-0 flex-1 flex-col md:flex-row">
        <nav className="flex shrink-0 gap-1 border-b border-border bg-panel p-2 md:w-16 md:flex-col md:border-b-0 md:border-r md:p-3 xl:w-52" aria-label="仓库目录">
          {sections.map((item) => {
            const Icon = item.icon;
            return (
              <Link
                key={item.id}
                to={repositoryRoute(item.id)}
                state={repositoryLocationState}
                aria-current={section === item.id ? 'page' : undefined}
                className={cn(
                  'flex h-10 flex-1 items-center justify-center gap-2.5 rounded-lg px-3 text-sm transition-colors active:translate-y-px md:flex-none xl:justify-start',
                  section === item.id
                    ? 'bg-accent-soft font-semibold text-accent shadow-[inset_0_0_0_1px_rgba(47,103,246,0.08)]'
                    : 'text-text-600 hover:bg-panel-muted hover:text-text-900',
                )}
              >
                <Icon className="h-[17px] w-[17px]" strokeWidth={1.75} />
                <span className="md:hidden xl:inline">{item.label}</span>
              </Link>
            );
          })}
        </nav>
        <section className="min-h-0 min-w-0 flex-1 overflow-hidden">{children}</section>
      </div>
    </main>
  );
}
