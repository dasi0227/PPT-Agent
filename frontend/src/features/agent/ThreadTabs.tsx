import React, { useState } from 'react';
import { Plus, X, ChevronDown, Trash2 } from 'lucide-react';
import { useProjectStore } from '../../stores/projectStore';
import { useThreadStore } from '../../stores/threadStore';
import { useRunStore, RunStatus } from '../../stores/runStore';
import { isDraftId } from '../../lib/draft';
import { cn } from '../../lib/utils';

function statusDot(status: RunStatus): string | null {
  switch (status) {
    case 'running': return 'bg-mode-normal animate-pulse';
    case 'needs_input': return 'bg-mode-ask animate-pulse';
    case 'error': return 'bg-mode-error';
    default: return null;
  }
}

export const ThreadTabs: React.FC = () => {
  const activeProjectId = useProjectStore((s) => s.activeProjectId);
  const {
    threadsByProjectId, draftThreadsByProjectId, openThreadIdsByProjectId, activeThreadIdByProjectId,
    openThread, closeThread, createDraftThread, deleteThread, setActiveThread,
  } = useThreadStore();
  const sessions = useRunStore((s) => s.sessions);
  const [historyOpen, setHistoryOpen] = useState(false);

  if (!activeProjectId) return null;

  const allThreads = [
    ...(threadsByProjectId[activeProjectId] || []),
    ...(draftThreadsByProjectId[activeProjectId] || []),
  ];
  const openIds = openThreadIdsByProjectId[activeProjectId] || [];
  const activeId = activeThreadIdByProjectId[activeProjectId] ?? null;
  const openThreads = openIds
    .map((id) => allThreads.find((t) => t.id === id))
    .filter((t): t is NonNullable<typeof t> => !!t);
  const historyThreads = allThreads.filter((t) => !openIds.includes(t.id) && !t.draft);

  const titleOf = (t: { title?: string }, i: number) => t.title || `对话 ${i + 1}`;

  const handleNew = () => {
    createDraftThread(activeProjectId, `新对话 ${allThreads.length + 1}`);
  };

  const handleDelete = async (threadId: string, label: string) => {
    if (!window.confirm(`删除对话「${label}」？历史记录将不可恢复。`)) return;
    try {
      await deleteThread(activeProjectId, threadId);
    } catch (err) {
      console.error('Failed to delete thread', err);
    }
  };

  return (
    <div className="flex items-center gap-1 h-9 border-b border-border px-2 bg-background overflow-x-auto">
      {openThreads.map((t, i) => {
        const isActive = t.id === activeId;
        const dot = statusDot((sessions[t.id]?.status) ?? 'idle');
        const draft = isDraftId(t.id);
        return (
          <div
            key={t.id}
            className={cn(
              'group flex items-center gap-1 h-7 pl-2 pr-1 rounded text-xs font-medium cursor-pointer transition-colors shrink-0',
              isActive ? 'bg-surface text-text-900 shadow-sm' : 'text-text-600 hover:bg-black/5'
            )}
            onClick={() => setActiveThread(activeProjectId, t.id)}
          >
            {dot && <span className={cn('w-1.5 h-1.5 rounded-full', dot)} />}
            <span className="max-w-[110px] truncate">{titleOf(t, i)}</span>
            {/* 真实 thread：删除入口（DELETE 后端，二次确认）。× 仅关闭标签。 */}
            {!draft && (
              <button
                type="button"
                aria-label={`删除 ${titleOf(t, i)}`}
                onClick={(e) => { e.stopPropagation(); handleDelete(t.id, titleOf(t, i)); }}
                className="p-0.5 rounded hover:bg-mode-error/15 hover:text-mode-error opacity-0 group-hover:opacity-100 transition-opacity"
              >
                <Trash2 className="w-3 h-3" />
              </button>
            )}
            <button
              type="button"
              aria-label={`关闭 ${titleOf(t, i)}`}
              onClick={(e) => { e.stopPropagation(); closeThread(activeProjectId, t.id); }}
              className="p-0.5 rounded hover:bg-black/10 opacity-0 group-hover:opacity-100 transition-opacity"
            >
              <X className="w-3 h-3" />
            </button>
          </div>
        );
      })}

      <button
        type="button"
        aria-label="新建对话"
        onClick={handleNew}
        className="shrink-0 p-1 rounded text-text-600 hover:bg-black/5 hover:text-text-900 transition-colors"
      >
        <Plus className="w-4 h-4" />
      </button>

      {historyThreads.length > 0 && (
        <div className="relative shrink-0 ml-auto">
          <button
            type="button"
            aria-label="历史对话"
            onClick={() => setHistoryOpen((v) => !v)}
            className="flex items-center gap-0.5 p-1 rounded text-text-600 hover:bg-black/5 text-xs"
          >
            历史 <ChevronDown className="w-3 h-3" />
          </button>
          {historyOpen && (
            <div className="absolute right-0 top-8 z-10 min-w-[160px] bg-surface border border-border rounded-md shadow-md py-1">
              {historyThreads.map((t, i) => (
                <button
                  key={t.id}
                  type="button"
                  onClick={() => { openThread(activeProjectId, t.id); setHistoryOpen(false); }}
                  className="w-full text-left px-3 py-1.5 text-xs text-text-600 hover:bg-black/5 truncate"
                >
                  {titleOf(t, i)}
                </button>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
};
