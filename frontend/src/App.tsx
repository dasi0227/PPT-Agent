import { AssetRepositoryPage } from './features/repository/AssetRepositoryPage';
import { useFileSettingsStore } from './stores/fileSettingsStore';
import { useAgentSettingsStore } from './stores/agentSettingsStore';
import { useShortcutStore } from './stores/shortcutStore';
import { showGlobalError } from './stores/toastStore';
import { ProjectHistoryDialogs } from './features/agent/ProjectHistoryControls';
import { useEffect, useState } from 'react';
import { createBrowserRouter, Outlet, RouterProvider, useLocation } from 'react-router-dom';
import { SettingsPage } from './features/settings/SettingsPage';
import { GlobalToasts } from './components/ui/GlobalToasts';
import { DeletionFeedback } from './features/workspace/DeletionFeedback';
import { GlobalModals } from './features/workspace/GlobalModals';
import { NotFoundPage } from './features/workspace/NotFoundPage';
import { WorkspaceRoute } from './features/workspace/WorkspaceRoute';

export function App() {
  useEffect(() => {
    const refresh = () => { void useAgentSettingsStore.getState().load().catch(() => {}); };
    const storage = (event: StorageEvent) => { if (event.key === 'ppt-agent-settings-updated') refresh(); };
    const visible = () => { if (document.visibilityState === 'visible') refresh(); };
    refresh();
    window.addEventListener('focus', refresh);
    window.addEventListener('storage', storage);
    window.addEventListener('model-settings-saved', refresh);
    document.addEventListener('visibilitychange', visible);
    return () => {
      window.removeEventListener('focus', refresh); window.removeEventListener('storage', storage);
      window.removeEventListener('model-settings-saved', refresh); document.removeEventListener('visibilitychange', visible);
    };
  }, []);
  useEffect(() => {
    const refresh = () => { void useFileSettingsStore.getState().load().catch(() => {}); };
    const storage = (event: StorageEvent) => { if (event.key === 'ppt-file-settings-updated') refresh(); };
    const visible = () => { if (document.visibilityState === 'visible') refresh(); };
    void useFileSettingsStore.getState().load().catch(() => showGlobalError('文件设置加载失败，可在设置中重试'));
    window.addEventListener('focus', refresh);
    window.addEventListener('storage', storage);
    document.addEventListener('visibilitychange', visible);
    return () => { window.removeEventListener('focus', refresh); window.removeEventListener('storage', storage); document.removeEventListener('visibilitychange', visible); };
  }, []);
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


  return <RouterProvider router={router} />;
}

function AppLayout() {
  const { pathname } = useLocation();
  const path = pathname.replace(/\/+$/, '').toLowerCase();
  const themeActive = path === '/assets' || path === '/assets/theme';
  const [themeVisited, setThemeVisited] = useState(themeActive);
  useEffect(() => { if (themeActive) setThemeVisited(true); }, [themeActive]);
  return <>
    {(themeActive || themeVisited) && (
      <div hidden={!themeActive} {...(!themeActive ? { inert: '' } : {})}>
        <AssetRepositoryPage key="Theme" type="theme" active={themeActive} />
      </div>
    )}
    <Outlet /><ProjectHistoryDialogs /><GlobalModals /><GlobalToasts />
    {path !== '/settings' && !themeActive && !path.startsWith('/assets/') && <DeletionFeedback />}
  </>;
}

const router = createBrowserRouter([{
  element: <AppLayout />,
  children: [
    { path: '/', element: <WorkspaceRoute /> },
    { path: '/projects/:projectId', element: <WorkspaceRoute /> },
    { path: '/assets', element: null },
    { path: '/assets/theme', element: null },
    { path: '/assets/component', element: <AssetRepositoryPage key="Component" type="component" /> },
    { path: '/assets/skill', element: <AssetRepositoryPage key="Skill" type="skill" /> },
    { path: '/assets/snippet', element: <AssetRepositoryPage key="Snippet" type="snippet" /> },
    { path: '/settings', element: <SettingsPage /> },
    { path: '*', element: <NotFoundPage /> },
  ],
}]);

export default App;
