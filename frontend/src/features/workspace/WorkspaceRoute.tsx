import React from 'react';
import { Navigate, useNavigate, useParams } from 'react-router-dom';
import { useProjectStore } from '../../stores/projectStore';
import { AppShell } from './AppShell';
import { homeRoute, projectRoute } from './routes';
import { useWorkspaceUrlState } from './useWorkspaceUrlState';
import { useThreadStore } from '../../stores/threadStore';
import { useRunStore } from '../../stores/runStore';
import { threadsApi } from '../../api/threads';
import { currentHistoryEpoch } from '../../api/client';
import type { HistoryEntry } from '../agent/historyHydrator';

const PROJECT_CHECK_INTERVAL_MS = 4_000;

function useVisibleProjectChecks(projectId: string | undefined) {
  React.useEffect(() => {
    if (!projectId) return;
    let canceled = false;
    let checking = false;
    const historySignatures = new Map<string, string>();
    const check = async () => {
      if (canceled || checking || document.visibilityState !== 'visible') return;
      checking = true;
      const epoch = currentHistoryEpoch();
      try {
        const threadIds = useThreadStore.getState().threadsByProjectId[projectId]?.map((thread) => thread.id) ?? [];
        await Promise.allSettled([
          useProjectStore.getState().checkProjectContent(projectId),
          ...threadIds.map(async (threadId) => {
            const startedWithSession = useRunStore.getState().sessions[threadId];
            const history = await threadsApi.history(threadId);
            if (canceled || epoch !== currentHistoryEpoch()
              || useRunStore.getState().sessions[threadId] !== startedWithSession) return;
            if (startedWithSession?.status === 'creating') return;
            const signature = JSON.stringify(history);
            if (historySignatures.get(threadId) === signature) {
              const session = useRunStore.getState().sessions[threadId];
              if (session?.activeRunId && !session.eventSourceClose && session.streamStatus === 'closed'
                && ['running', 'waiting', 'recovering'].includes(session.status)) {
                void useRunStore.getState().reconcileRun(threadId, session.activeRunId, session.lastEventId, projectId);
              }
              return;
            }
            if (useRunStore.getState().syncThreadHistory(threadId, history as unknown as HistoryEntry[], projectId)) {
              historySignatures.set(threadId, signature);
            }
          }),
        ]);
      } finally {
        checking = false;
      }
    };
    const timer = window.setInterval(() => void check(), PROJECT_CHECK_INTERVAL_MS);
    document.addEventListener('visibilitychange', check);
    window.addEventListener('focus', check);
    return () => {
      canceled = true;
      window.clearInterval(timer);
      document.removeEventListener('visibilitychange', check);
      window.removeEventListener('focus', check);
    };
  }, [projectId]);
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
  useVisibleProjectChecks(projectId);

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
