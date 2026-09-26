import { HomeLogo } from '../../components/ui/HomeLogo';
import type { ReactNode } from 'react';
import { BookOpenText, Component, NotebookText, Palette, RefreshCw } from 'lucide-react';
import { Link, useLocation } from 'react-router-dom';
import { IconButton } from '../../components/ui/primitives';
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
  { id: 'snippet', label: '短语', icon: NotebookText },
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
      <header className="flex h-12 shrink-0 items-center border-b border-border-strong bg-surface px-2 shadow-[0_1px_0_rgba(255,255,255,0.75)]">
        <div className="flex min-w-0 items-center gap-2 px-2 text-base font-bold">
          <img src="/logo.jpg" alt="" className="h-10 w-10 rounded-sm object-cover" />
          <span className="hidden truncate sm:inline">Dasi PPT Agent</span>
        </div>
        <span className="mx-2 h-5 w-px bg-border" aria-hidden="true" />
        <span className="min-w-0 flex-1 truncate text-base font-bold text-text-900">仓库</span>
        <IconButton label="刷新仓库" expandableLabel="刷新" onClick={onRefresh} className="active:translate-y-px">
          <RefreshCw className="h-4 w-4" strokeWidth={1.75} />
        </IconButton>
        <Link
          to={returnTo}
          className="expandable-icon-button ml-1 inline-flex h-8 shrink-0 items-center justify-center rounded-md text-text-600 ui-interactive focus-visible:outline-none active:translate-y-px"
          title="返回主页"
          aria-label="返回主页"
        >
          <HomeLogo />
          <span aria-hidden="true" className="expandable-icon-label">主页</span>
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
                    ? 'ui-selected font-semibold'
                    : 'text-text-600 ui-interactive',
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
