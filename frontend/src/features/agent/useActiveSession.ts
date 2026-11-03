import { useMemo } from 'react';
import { useProjectStore } from '../../stores/projectStore';
import { useThreadStore } from '../../stores/threadStore';
import { useRunStore, UNLOADED_SESSION, type RunSession } from '../../stores/runStore';

const EMPTY_SESSION: RunSession = Object.freeze({ ...UNLOADED_SESSION, loadStatus: 'ready', isEmpty: true });

export function useActiveThreadId(): string | null {
  const projectId = useProjectStore(state => state.activeProjectId);
  return useThreadStore(state => projectId ? state.activeThreadIdByProjectId[projectId] ?? null : null);
}

// Loading belongs to the workspace/session owner. All consumers are selectors.
export function useActiveSession(): RunSession {
  const threadId = useActiveThreadId();
  const projectId = useProjectStore(state => state.activeProjectId);
  const noThreads = useThreadStore(state => Boolean(projectId && state.threadsByProjectId[projectId]?.length === 0 && !state.errorByProjectId[projectId]));
  const threadError = useThreadStore(state => projectId ? state.errorByProjectId[projectId] : null);
  const session = useRunStore(state => threadId ? state.sessions[threadId] ?? UNLOADED_SESSION : noThreads ? EMPTY_SESSION : UNLOADED_SESSION);
  return useMemo(() => !threadId && threadError
    ? { ...UNLOADED_SESSION, loadStatus: 'error' as const, error: new Error(threadError) }
    : session, [session, threadError, threadId]);
}
