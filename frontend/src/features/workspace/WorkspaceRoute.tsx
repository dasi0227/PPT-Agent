import React from 'react';
import { Navigate, useNavigate, useParams } from 'react-router-dom';
import { useProjectStore } from '../../stores/projectStore';
import { useThreadStore } from '../../stores/threadStore';
import { AppShell } from './AppShell';
import { homeRoute } from './routes';
import { useWorkspaceUrlState } from './useWorkspaceUrlState';

function syncProjectRoute(projectId: string | undefined) {
  if (!projectId) {
    useProjectStore.setState({ activeProjectId: null });
    return;
  }

  const state = useProjectStore.getState();
  useProjectStore.setState({
    activeProjectId: projectId,
    openProjectIds: state.openProjectIds.includes(projectId)
      ? state.openProjectIds
      : [...state.openProjectIds, projectId],
  });
}

export function WorkspaceRoute() {
  const { projectId } = useParams();
  const navigate = useNavigate();
  const loadProjects = useProjectStore((state) => state.loadProjects);
  const loadProjectSlides = useProjectStore((state) => state.loadProjectSlides);
  const loadThreads = useThreadStore((state) => state.loadThreads);
  useWorkspaceUrlState(projectId);

  React.useLayoutEffect(() => {
    syncProjectRoute(projectId);
  }, [projectId]);

  React.useEffect(() => {
    if (!projectId) return;

    let canceled = false;
    const validateAndLoad = async () => {
      await loadProjects();
      if (canceled) return;

      const state = useProjectStore.getState();
      const exists = state.projects.some((project) => project.id === projectId);
      if (!exists) {
        navigate(homeRoute, { replace: true });
        return;
      }

      void loadProjectSlides(projectId);
      void loadThreads(projectId);
    };

    void validateAndLoad();
    return () => {
      canceled = true;
    };
  }, [loadProjectSlides, loadProjects, loadThreads, navigate, projectId]);

  return <AppShell />;
}

export function UnknownRouteRedirect() {
  return <Navigate to={homeRoute} replace />;
}
