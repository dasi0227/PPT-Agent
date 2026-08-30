import type { ReactNode } from 'react';
import { BookOpenText, Component, Home, Palette, RefreshCw } from 'lucide-react';
import { Link, useNavigate } from 'react-router-dom';
import { cn } from '../../lib/utils';
import { homeRoute, repositoryRoute, type RepositorySection } from '../workspace/routes';

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
  title,
  onRefresh,
  children,
}: {
  section: RepositorySection;
  title: string;
  onRefresh: () => void;
  children: ReactNode;
}) {
  const navigate = useNavigate();
  return (
    <main className="flex min-h-[100dvh] min-w-0 flex-col overflow-hidden bg-workspace text-text-900">
      <header className="flex h-12 shrink-0 items-center border-b border-border-strong bg-panel px-3">
        <button
          type="button"
          onClick={() => navigate(homeRoute)}
          className="mr-3 flex min-w-0 items-center gap-2 font-bold"
          aria-label="返回主工作台"
        >
          <img src="/logo.jpg" alt="" className="h-8 w-8 rounded-sm object-cover" />
          <span className="hidden truncate sm:inline">Dasi PPT Agent</span>
        </button>
        <span className="min-w-0 flex-1 truncate text-sm font-semibold">{title}</span>
        <button type="button" onClick={onRefresh} className="grid h-8 w-8 place-items-center rounded-md text-text-600 hover:bg-panel-muted" title="刷新仓库" aria-label="刷新仓库">
          <RefreshCw className="h-4 w-4" />
        </button>
        <Link to={homeRoute} className="grid h-8 w-8 place-items-center rounded-md text-text-600 hover:bg-panel-muted" title="返回首页" aria-label="返回首页">
          <Home className="h-4 w-4" />
        </Link>
      </header>
      <div className="flex min-h-0 flex-1 flex-col md:flex-row">
        <nav className="flex shrink-0 border-b border-border bg-panel p-2 md:w-44 md:flex-col md:border-b-0 md:border-r md:p-3" aria-label="个人仓库">
          <div className="hidden px-2 pb-2 pt-1 text-xs font-semibold text-text-400 md:block">个人仓库</div>
          {sections.map((item) => {
            const Icon = item.icon;
            return (
              <Link
                key={item.id}
                to={repositoryRoute(item.id)}
                aria-current={section === item.id ? 'page' : undefined}
                className={cn(
                  'flex h-9 flex-1 items-center justify-center gap-2 rounded-md px-3 text-sm md:flex-none md:justify-start',
                  section === item.id ? 'bg-accent-soft font-semibold text-accent' : 'text-text-600 hover:bg-panel-muted',
                )}
              >
                <Icon className="h-4 w-4" />
                <span>{item.label}</span>
              </Link>
            );
          })}
        </nav>
        <section className="min-h-0 min-w-0 flex-1 overflow-hidden">{children}</section>
      </div>
    </main>
  );
}
