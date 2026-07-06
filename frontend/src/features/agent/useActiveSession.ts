import { useProjectStore } from '../../stores/projectStore';
import { useThreadStore } from '../../stores/threadStore';
import { useRunStore, RunSession, IDLE_SESSION } from '../../stores/runStore';

// 当前聚焦 project 的活跃 threadId（可能为 null）。
export function useActiveThreadId(): string | null {
  const activeProjectId = useProjectStore((s) => s.activeProjectId);
  const activeThreadIdByProjectId = useThreadStore((s) => s.activeThreadIdByProjectId);
  if (!activeProjectId) return null;
  return activeThreadIdByProjectId[activeProjectId] ?? null;
}

// 当前聚焦 thread 的 RunSession（缺省返回稳定 IDLE 空会话）。
export function useActiveSession(): RunSession {
  const threadId = useActiveThreadId();
  const sessions = useRunStore((s) => s.sessions);
  if (!threadId) return IDLE_SESSION;
  return sessions[threadId] ?? IDLE_SESSION;
}
