import { useShortcutStore } from './stores/shortcutStore';
import { showGlobalError } from './stores/toastStore';
import { ProjectHistoryDialogs } from './features/agent/ProjectHistoryControls';
import { useEffect, useState } from 'react';
import { createBrowserRouter, Outlet, RouterProvider, useLocation } from 'react-router-dom';
import { SettingsPage } from './features/settings/SettingsPage';
import { GlobalToasts } from './components/ui/GlobalToasts';
import { GlobalModals } from './features/workspace/GlobalModals';
import { UnknownRouteRedirect, WorkspaceRoute } from './features/workspace/WorkspaceRoute';
import { useRunStore } from './stores/runStore';
import { useGitCommitStore } from './stores/gitCommitStore';
import { ThemeRepositoryPage } from './features/repository/ThemeRepositoryPage';
import { ComponentRepositoryPage } from './features/repository/ComponentRepositoryPage';
import { SkillRepositoryPage } from './features/repository/SkillRepositoryPage';
import { SnippetRepositoryPage } from './features/repository/SnippetRepositoryPage';

export function App() {
  useEffect(() => {
    const refresh = () => { void useShortcutStore.getState().load().catch(() => {}); };
    const storage = (event: StorageEvent) => { if (event.key === 'ppt-shortcuts-updated') refresh(); };
    const visible = () => { if (document.visibilityState === 'visible') refresh(); };
    void useShortcutStore.getState().load().catch(() => showGlobalError('快捷键设置加载失败，可在设置中重试'));
    window.addEventListener('focus', refresh);
    window.addEventListener('storage', storage);
    document.addEventListener('visibilitychange', visible);
    return () => { window.removeEventListener('focus', refresh); window.removeEventListener('storage', storage); document.removeEventListener('visibilitychange', visible); };
  }, []);
  useEffect(() => {
    void useRunStore.getState().recoverPersistedRuns();
    void useGitCommitStore.getState().recover();
    return () => {
      const sessions = useRunStore.getState().sessions;
      Object.values(sessions).forEach((session) => session.eventSourceClose?.());
      useGitCommitStore.getState().closeAll();
    };
  }, []);

  return <RouterProvider router={router} />;
}

function AppLayout() {
  const { pathname } = useLocation();
  const themeActive = pathname.replace(/\/+$/, '').toLowerCase() === '/warehouse/theme';
  const [themeVisited, setThemeVisited] = useState(themeActive);
  useEffect(() => { if (themeActive) setThemeVisited(true); }, [themeActive]);
  return <>
    {(themeActive || themeVisited) && (
      <div hidden={!themeActive} {...(!themeActive ? { inert: '' } : {})}>
        <ThemeRepositoryPage active={themeActive} />
      </div>
    )}
    <Outlet /><ProjectHistoryDialogs /><GlobalModals /><GlobalToasts />
  </>;
}

const router = createBrowserRouter([{
  element: <AppLayout />,
  children: [
    { path: '/', element: <WorkspaceRoute /> },
    { path: '/projects/:projectId', element: <WorkspaceRoute /> },
    { path: '/warehouse/theme', element: null },
    { path: '/warehouse/component', element: <ComponentRepositoryPage /> },
    { path: '/warehouse/skill', element: <SkillRepositoryPage /> },
    { path: '/warehouse/snippet', element: <SnippetRepositoryPage /> },
    { path: '/settings', element: <SettingsPage /> },
    { path: '*', element: <UnknownRouteRedirect /> },
  ],
}]);

export default App;
