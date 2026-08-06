import { useEffect } from 'react';
import { BrowserRouter, Route, Routes } from 'react-router-dom';
import { GlobalModals } from './features/workspace/GlobalModals';
import { UnknownRouteRedirect, WorkspaceRoute } from './features/workspace/WorkspaceRoute';
import { useRunStore } from './stores/runStore';

export function App() {
  useEffect(() => {
    void useRunStore.getState().recoverPersistedRuns();
    return () => {
      const sessions = useRunStore.getState().sessions;
      Object.values(sessions).forEach((session) => session.eventSourceClose?.());
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
    </BrowserRouter>
  );
}

export default App;
