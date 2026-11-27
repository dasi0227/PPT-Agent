import React from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useProjectStore } from '../../stores/projectStore';
import { AppShell } from './AppShell';
import { homeRoute, projectRoute } from './routes';
import { useWorkspaceUrlState } from './useWorkspaceUrlState';
import { useThreadStore } from '../../stores/threadStore';
import { useSessionStore } from '../../stores/sessionStore';
import { showGlobalError } from '../../stores/toastStore';

function useSessionSync(projectId: string | undefined) {
  const threadId = useThreadStore(state => projectId ? state.activeThreadIdByProjectId[projectId] : null);
  React.useEffect(() => {
    if (!projectId) return;
    const refresh = (areas: readonly unknown[]) => {
      const projectStore = useProjectStore.getState();
      if (areas.includes('content')) void projectStore.loadProjectContent(projectId);
      if (areas.includes('appearance')) void projectStore.loadProjectAppearance(projectId)
        .catch(cause => showGlobalError(cause instanceof Error ? cause.message : '项目外观加载失败'));
      if (areas.includes('sessions')) void useThreadStore.getState().loadThreads(projectId);
    };
    const refreshAll = () => refresh(['content', 'appearance', 'sessions']);
    const release = threadId ? useSessionStore.getState().retain(threadId, projectId, refresh) : undefined;
    const focus = () => {
      if (document.visibilityState === 'visible') refreshAll();
    };
    window.addEventListener('focus', focus);
    document.addEventListener('visibilitychange', focus);
    return () => {
      release?.();
      window.removeEventListener('focus', focus);
      document.removeEventListener('visibilitychange', focus);
    };
  }, [projectId, threadId]);
}

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
  useSessionSync(projectId);

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
    let canceled = false;
    const validateAndLoad = async () => {
      const loaded = await loadProjects();
      if (!loaded || canceled || !projectId) return;

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
