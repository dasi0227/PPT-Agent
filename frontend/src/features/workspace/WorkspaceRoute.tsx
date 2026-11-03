import React from 'react';
import { Navigate, useNavigate, useParams } from 'react-router-dom';
import { useProjectStore } from '../../stores/projectStore';
import { AppShell } from './AppShell';
import { homeRoute, projectRoute } from './routes';
import { useWorkspaceUrlState } from './useWorkspaceUrlState';
import { useThreadStore } from '../../stores/threadStore';
import { useSessionStore } from '../../stores/sessionStore';
import { compareRevision } from '../../api/session';

const PROJECT_CONTENT_CHECK_INTERVAL_MS = 4_000;

function useSessionSync(projectId: string | undefined) {
  const threadId = useThreadStore(state => projectId ? state.activeThreadIdByProjectId[projectId] : null);
  React.useEffect(() => {
    if (!projectId) return;
    const check = () => {
      if (document.visibilityState === 'visible') void useProjectStore.getState().checkProjectContent(projectId);
    };
    const timer = window.setInterval(check, PROJECT_CONTENT_CHECK_INTERVAL_MS);
    window.addEventListener('focus', check);
    document.addEventListener('visibilitychange', check);
    return () => {
      window.clearInterval(timer);
      window.removeEventListener('focus', check);
      document.removeEventListener('visibilitychange', check);
    };
  }, [projectId]);

  React.useEffect(() => {
    if (!projectId || !threadId) return;
    const release = useSessionStore.getState().retain(threadId, projectId);
    let timer: ReturnType<typeof setTimeout> | undefined;
    const unsubscribe = useSessionStore.subscribe((next, previous) => {
      const state = next.sessions[threadId], before = previous.sessions[threadId];
      if (!state || !before?.appliedSeq || state.sceneRevision !== before.sceneRevision || state.appliedSeq === before.appliedSeq) return;
      const fileChange = Object.values(state.itemsById).some(item => item.kind === 'tool'
        && item.summary.status === 'completed' && compareRevision(item.updated_seq, before.appliedSeq!) > 0);
      const ended = before.thread?.active_run_id && state.thread?.active_run_id !== before.thread.active_run_id;
      if (!fileChange && !ended) return;
      clearTimeout(timer);
      timer = setTimeout(() => void useProjectStore.getState().checkProjectContent(projectId), 100);
    });
    const focus = () => {
      if (document.visibilityState !== 'visible') return;
      void useSessionStore.getState().reconnect(threadId);
    };
    window.addEventListener('focus', focus);
    document.addEventListener('visibilitychange', focus);
    return () => {
      clearTimeout(timer); unsubscribe(); release();
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
