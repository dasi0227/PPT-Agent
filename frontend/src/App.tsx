import { useEffect } from 'react';
import { BrowserRouter, Route, Routes } from 'react-router-dom';
import { GlobalErrorToasts } from './components/ui/GlobalErrorToasts';
import { GlobalModals } from './features/workspace/GlobalModals';
import { UnknownRouteRedirect, WorkspaceRoute } from './features/workspace/WorkspaceRoute';
import { useRunStore } from './stores/runStore';
import { useGitCommitStore } from './stores/gitCommitStore';

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
        <Route path="*" element={<UnknownRouteRedirect />} />
      </Routes>
      <GlobalModals />
      <GlobalErrorToasts />
    </BrowserRouter>
  );
}

export default App;
