import { useEffect } from 'react';
import { useProjectStore } from '../../stores/projectStore';
import { useThreadStore } from '../../stores/threadStore';
import { useRunStore, RunSession, IDLE_SESSION } from '../../stores/runStore';
import { threadsApi } from '../../api/threads';
import { isDraftId } from '../../lib/draft';
import { hydrateFromHistory, HistoryEntry } from './historyHydrator';

// 当前聚焦 project 的活跃 threadId（可能为 null）。
export function useActiveThreadId(): string | null {
  const activeProjectId = useProjectStore((s) => s.activeProjectId);
  const activeThreadIdByProjectId = useThreadStore((s) => s.activeThreadIdByProjectId);
  if (!activeProjectId) return null;
  return activeThreadIdByProjectId[activeProjectId] ?? null;
}

// 当前聚焦 thread 的 RunSession（缺省返回稳定 IDLE 空会话）。
// 副作用：在 threadId 变化时按需 replay 后端 history（仅当前端 timelineItems 为空且非草稿）。
export function useActiveSession(): RunSession {
  const threadId = useActiveThreadId();
  const sessions = useRunStore((s) => s.sessions);

  useEffect(() => {
    if (!threadId || isDraftId(threadId)) return;
    // 空态才 replay：运行时 in-memory 优先，防止刷新覆盖已有 SSE 增量。
    const current = useRunStore.getState().sessions[threadId];
    if (current && current.timelineItems.length > 0) return;
    threadsApi.history(threadId)
      .then((entries) => {
        if (!entries || entries.length === 0) return;
        const items = hydrateFromHistory(entries as unknown as HistoryEntry[]);
        useRunStore.getState().hydrateTimeline(threadId, items);
      })
      .catch((err) => console.warn('history replay failed', err));
  }, [threadId]);

  if (!threadId) return IDLE_SESSION;
  return sessions[threadId] ?? IDLE_SESSION;
}
