import { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { useDeletionStore } from '../../stores/deletionStore';
import { projectRoute } from './routes';

export function DeletionFeedback() {
  const navigate = useNavigate();
  const restored = useDeletionStore((state) => state.restored);
  useEffect(() => {
    if (!restored) return;
    useDeletionStore.setState({ restored: null });
    navigate(projectRoute(restored.projectId), { replace: true });
  }, [navigate, restored]);
  return null;
}
