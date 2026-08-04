import React from 'react';
import { useProjectStore } from '../../stores/projectStore';
import { useThreadStore } from '../../stores/threadStore';
import { cn } from '../../lib/utils';
import { Loader2, MoreHorizontal, Plus } from 'lucide-react';
import { ThreadMenu } from './ThreadMenu';

export const ThreadTabs: React.FC = () => {
  const { activeProjectId } = useProjectStore();
  const { displayThreads, activeThreadIdByProjectId, setActiveThread, createThread, errorByProjectId } = useThreadStore();
  const [creating, setCreating] = React.useState(false);

  if (!activeProjectId) return null;

  const threads = displayThreads(activeProjectId);
  const activeId = activeThreadIdByProjectId[activeProjectId];
  const error = errorByProjectId[activeProjectId];

  const handleCreate = async () => {
    if (creating) return;
    setCreating(true);
    try {
      await createThread(activeProjectId);
    } finally {
      setCreating(false);
    }
  };

  return (
    <div className="flex flex-col border-b border-border bg-surface">
      {error && <div role="alert" className="border-b border-danger/20 bg-danger-soft px-3 py-1.5 text-xs text-danger">{error}。请重新打开项目。</div>}
      <div className="flex items-center px-2 py-1 select-none min-h-[36px]">
        <div className="flex min-w-0 flex-1 items-center overflow-x-auto">
          {threads.length === 0 ? (
            <div className="px-1 py-1.5 text-xs text-text-400">
              暂无会话
            </div>
          ) : (
            threads.map(th => {
              const isActive = th.id === activeId;
              return (
                <div
                  key={th.id}
                  role="tab"
                  aria-selected={isActive}
                  tabIndex={0}
                  onClick={() => setActiveThread(activeProjectId, th.id)}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter' || event.key === ' ') {
                      event.preventDefault();
                      setActiveThread(activeProjectId, th.id);
                    }
                  }}
                  className={cn(
                    "group px-3 py-1.5 rounded-md text-xs font-medium cursor-pointer transition-colors flex items-center gap-1.5 min-w-[100px] max-w-[160px] focus-visible:outline-none",
                    isActive ? "bg-black/5 text-text-900" : "text-text-600 hover:bg-black/5"
                  )}
                >
                  <span className="truncate flex-1">{th.title || '新会话'}</span>

                  <ThreadMenu projectId={activeProjectId} thread={th}>
                    <button
                      type="button"
                      aria-label="会话选项"
                      onClick={(e) => e.stopPropagation()}
                      className="p-0.5 rounded hover:bg-black/10 opacity-0 group-hover:opacity-100 group-focus-within:opacity-100 transition-opacity"
                    >
                      <MoreHorizontal className="w-3 h-3 text-text-400" />
                    </button>
                  </ThreadMenu>
                </div>
              );
            })
          )}
        </div>
        <button
          type="button"
          onClick={handleCreate}
          disabled={creating}
          aria-label="新建会话"
          title="新建会话"
          className="ml-1 flex h-7 w-7 shrink-0 items-center justify-center rounded-md text-text-600 transition-colors hover:bg-black/5 hover:text-text-900 disabled:cursor-not-allowed disabled:opacity-40"
        >
          {creating ? <Loader2 className="h-4 w-4 animate-spin" /> : <Plus className="h-4 w-4" />}
        </button>
      </div>
    </div>
  );
};
