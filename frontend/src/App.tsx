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
import { PromptRepositoryPage } from './features/repository/PromptRepositoryPage';

export function App() {
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
    { path: '/warehouse/prompt', element: <PromptRepositoryPage /> },
    { path: '/settings', element: <SettingsPage /> },
    { path: '*', element: <UnknownRouteRedirect /> },
  ],
}]);

export default App;
