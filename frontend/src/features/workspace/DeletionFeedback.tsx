import { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { recoverDeletionNotices, useDeletionStore } from '../../stores/deletionStore';
import { projectRoute } from './routes';

export function DeletionFeedback() {
  const navigate = useNavigate();
  const restored = useDeletionStore((state) => state.restored);
  useEffect(() => {
    const refresh = () => { void recoverDeletionNotices().catch(() => {}); };
    const visible = () => { if (document.visibilityState === 'visible') refresh(); };
    refresh();
    window.addEventListener('focus', refresh);
    document.addEventListener('visibilitychange', visible);
    return () => { window.removeEventListener('focus', refresh); document.removeEventListener('visibilitychange', visible); };
  }, []);
  useEffect(() => {
    if (!restored) return;
    useDeletionStore.setState({ restored: null });
    navigate(projectRoute(restored.projectId), { replace: true });
  }, [navigate, restored]);
  return null;
}
