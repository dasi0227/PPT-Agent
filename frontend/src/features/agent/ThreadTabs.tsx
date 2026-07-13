import React from 'react';
import { useProjectStore } from '../../stores/projectStore';
import { useThreadStore } from '../../stores/threadStore';
import { cn } from '../../lib/utils';
import { MessageSquare, MoreHorizontal } from 'lucide-react';
import { ThreadMenu } from './ThreadMenu';

export const ThreadTabs: React.FC = () => {
  const { activeProjectId } = useProjectStore();
  const { displayThreads, activeThreadIdByProjectId, setActiveThread } = useThreadStore();

  if (!activeProjectId || activeProjectId === 'new-pending') return null;

  const threads = displayThreads(activeProjectId);
  const activeId = activeThreadIdByProjectId[activeProjectId];

  return (
    <div className="flex flex-col border-b border-border bg-surface">
      <div className="flex items-center px-2 py-1 overflow-x-auto select-none">
        {threads.map(th => {
          const isActive = th.id === activeId;
          return (
            <div
              key={th.id}
              onClick={() => setActiveThread(activeProjectId, th.id)}
              className={cn(
                "group px-3 py-1.5 rounded-md text-xs font-medium cursor-pointer transition-colors flex items-center gap-1.5 min-w-[100px] max-w-[160px]",
                isActive ? "bg-black/5 text-text-900" : "text-text-600 hover:bg-black/5"
              )}
            >
              <MessageSquare className="w-3.5 h-3.5 shrink-0" />
              <span className="truncate flex-1">{th.title || '新会话'}</span>
              
              <ThreadMenu projectId={activeProjectId} thread={th}>
                <button
                  type="button"
                  aria-label="会话选项"
                  onClick={(e) => e.stopPropagation()}
                  className="p-0.5 rounded hover:bg-black/10 opacity-0 group-hover:opacity-100 transition-opacity"
                >
                  <MoreHorizontal className="w-3 h-3 text-text-400" />
                </button>
              </ThreadMenu>
            </div>
          );
        })}
      </div>
    </div>
  );
};
