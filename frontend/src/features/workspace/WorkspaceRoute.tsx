import React from 'react';
import { Navigate, useNavigate, useParams } from 'react-router-dom';
import { useProjectStore } from '../../stores/projectStore';
import { AppShell } from './AppShell';
import { homeRoute, projectRoute } from './routes';
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
  const activeProjectId = useProjectStore((state) => state.activeProjectId);
  const openProjectIds = useProjectStore((state) => state.openProjectIds);
  useWorkspaceUrlState(projectId);

  React.useLayoutEffect(() => {
    syncProjectRoute(projectId);
  }, [projectId]);

  React.useEffect(() => {
    if (!projectId) return;
    const state = useProjectStore.getState();
    if (state.openProjectIds.includes(projectId)) return;

    const nextActive = state.activeProjectId && state.openProjectIds.includes(state.activeProjectId)
      ? state.activeProjectId
      : state.openProjectIds[state.openProjectIds.length - 1] ?? null;
    navigate(nextActive ? projectRoute(nextActive) : homeRoute, { replace: true });
  }, [activeProjectId, navigate, openProjectIds, projectId]);

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
      }
    };

    void validateAndLoad();
    return () => {
      canceled = true;
    };
  }, [loadProjects, navigate, projectId]);

  return <AppShell />;
}

export function UnknownRouteRedirect() {
  return <Navigate to={homeRoute} replace />;
}
