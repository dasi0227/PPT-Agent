import { useEffect } from 'react';
import { AppShell } from './features/workspace/AppShell';
import { GlobalModals } from './features/workspace/GlobalModals';
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
    <>
      <AppShell />
      <GlobalModals />
    </>
  );
}

export default App;
