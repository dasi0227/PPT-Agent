import { Archive, RefreshCw, Settings } from 'lucide-react';
import { Link, useLocation } from 'react-router-dom';
import { homeRoute, projectRoute, repositoryRoute } from '../../features/workspace/routes';
import { cn } from '../../lib/utils';
import { useProjectStore } from '../../stores/projectStore';
import { AppearanceActions } from './AppearanceActions';
import { HomeLogo } from './HomeLogo';
import { IconButton } from './primitives';

export function HeaderActions({
  onRefresh,
  refreshLabel = '刷新',
  refreshing = false,
  refreshDisabled = false,
  navigationDisabled = false,
  className,
}: {
  onRefresh: () => void;
  refreshLabel?: string;
  refreshing?: boolean;
  refreshDisabled?: boolean;
  navigationDisabled?: boolean;
  className?: string;
}) {
  const { pathname, search, hash, state } = useLocation();
  const activeProjectId = useProjectStore(value => value.activeProjectId);
  const openProjectIds = useProjectStore(value => value.openProjectIds);
  const path = pathname.toLowerCase().replace(/\/+$/, '') || homeRoute;
  const inSettings = path === '/settings' || path.startsWith('/settings/');
  const inRepository = path === '/warehouse' || path.startsWith('/warehouse/');
  const inWorkspace = path.startsWith('/projects/');
  const returnProjectId = activeProjectId ?? openProjectIds[openProjectIds.length - 1];
  const previousWorkspace = typeof state?.returnTo === 'string' && state.returnTo.startsWith('/projects/')
    ? state.returnTo : null;
  const workspaceReturnTo = inWorkspace ? `${pathname}${search}${hash}`
    : previousWorkspace ?? (returnProjectId ? projectRoute(returnProjectId) : null);
  const navigationState = workspaceReturnTo ? { returnTo: workspaceReturnTo } : undefined;
  const links = [
    { to: '/settings', label: '设置', text: '设置', icon: <Settings size={16} aria-hidden="true" />, hidden: inSettings },
    { to: repositoryRoute('theme'), label: '仓库', text: '仓库', icon: <Archive size={16} aria-hidden="true" />, hidden: inRepository },
    { to: workspaceReturnTo, label: '返回主页', text: '主页', icon: <HomeLogo />, hidden: inWorkspace },
  ];

  return (
    <div className={cn('flex shrink-0 items-center gap-2', className)}>
      <AppearanceActions />
      <IconButton label={refreshLabel} expandableLabel="刷新" onClick={onRefresh}
        disabled={refreshDisabled || refreshing} aria-busy={refreshing}>
        <RefreshCw size={16} aria-hidden="true" className={refreshing ? 'animate-spin motion-reduce:animate-none' : undefined} />
      </IconButton>
      {links.filter(link => !link.hidden).map(link => link.to ? (
        <Link key={link.text} to={link.to} state={navigationState} aria-label={link.label} title={link.label}
          aria-disabled={navigationDisabled || undefined} tabIndex={navigationDisabled ? -1 : undefined}
          onClick={navigationDisabled ? event => event.preventDefault() : undefined}
          className={cn(
            'inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-[var(--radius-control)] text-text-600 ui-interactive expandable-icon-button',
            navigationDisabled && 'cursor-not-allowed opacity-40',
          )}>
          {link.icon}
          <span aria-hidden="true" className="expandable-icon-label">{link.text}</span>
        </Link>
      ) : (
        <IconButton key={link.text} label={link.label} expandableLabel={link.text} disabled={navigationDisabled}
          onClick={() => document.dispatchEvent(new CustomEvent('open-project-picker'))}>
          {link.icon}
        </IconButton>
      ))}
    </div>
  );
}
