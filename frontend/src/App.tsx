import { useEffect } from 'react';
import { BrowserRouter, Route, Routes } from 'react-router-dom';
import { GlobalToasts } from './components/ui/GlobalToasts';
import { GlobalModals } from './features/workspace/GlobalModals';
import { UnknownRouteRedirect, WorkspaceRoute } from './features/workspace/WorkspaceRoute';
import { useRunStore } from './stores/runStore';
import { useGitCommitStore } from './stores/gitCommitStore';
import { ThemeRepositoryPage } from './features/repository/ThemeRepositoryPage';
import { ComponentRepositoryPage } from './features/repository/ComponentRepositoryPage';
import { SkillRepositoryPage } from './features/repository/SkillRepositoryPage';

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

  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<WorkspaceRoute />} />
        <Route path="/projects/:projectId" element={<WorkspaceRoute />} />
        <Route path="/warehouse/theme" element={<ThemeRepositoryPage />} />
        <Route path="/warehouse/component" element={<ComponentRepositoryPage />} />
        <Route path="/warehouse/skill" element={<SkillRepositoryPage />} />
        <Route path="*" element={<UnknownRouteRedirect />} />
      </Routes>
      <GlobalModals />
      <GlobalToasts />
    </BrowserRouter>
  );
}

export default App;
