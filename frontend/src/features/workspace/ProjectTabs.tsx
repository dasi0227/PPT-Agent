import React, { useEffect } from 'react';
import { useProjectStore } from '../../stores/projectStore';
import { useActiveSession } from '../agent/useActiveSession';
import { cn } from '../../lib/utils';
import { Loader2, Plus, X } from 'lucide-react';

export const ProjectTabs: React.FC = () => {
  const { projects, activeProjectId, selectProject, loadingProjects, loadProjects, createDraftProject, deleteProject } = useProjectStore();
  const { status: runStatus } = useActiveSession();

  useEffect(() => {
    loadProjects();
  }, [loadProjects]);

  const handleCreate = () => {
    createDraftProject();
  };

  const handleRemove = async (proj: { id: string; title: string; draft?: boolean }) => {
    // 草稿：静默丢弃；真实：二次确认（删历史不可逆）。
    if (!proj.draft && !window.confirm(`删除演示文稿「${proj.title || 'Untitled'}」？此操作不可撤销。`)) return;
    try {
      await deleteProject(proj.id);
    } catch (err) {
      console.error('Failed to delete project', err);
    }
  };

  return (
    <div className="flex items-center h-12 bg-background border-b border-border-strong px-2 overflow-x-auto select-none">
      <div className="flex-shrink-0 mr-4 font-bold text-text-900 px-2 flex items-center">
        <span className="w-5 h-5 bg-mode-normal rounded-sm mr-2 inline-block" />
        M7 Studio
      </div>
      
      {loadingProjects && projects.length === 0 ? (
        <Loader2 className="w-4 h-4 animate-spin text-text-400" />
      ) : (
        <div className="flex items-end h-full gap-1">
          {projects.map((proj) => {
            const isActive = proj.id === activeProjectId;
            return (
              <div
                key={proj.id}
                onClick={() => selectProject(proj.id)}
                className={cn(
                  "group h-10 px-3 pl-4 rounded-t-md text-sm font-medium transition-colors border border-b-0 flex items-center gap-1 cursor-pointer",
                  isActive
                    ? "bg-surface text-text-900 border-border-strong border-b-transparent shadow-sm relative top-[1px]"
                    : "bg-background text-text-600 border-transparent hover:bg-black/5"
                )}
              >
                <span className="truncate max-w-[160px]">{proj.title || 'Untitled Project'}</span>
                {proj.draft && (
                  <span className="text-[9px] uppercase text-text-400 border border-border rounded px-1 py-px">草稿</span>
                )}
                {isActive && runStatus === 'running' && (
                  <span className="inline-block w-2 h-2 rounded-full bg-mode-normal animate-pulse" />
                )}
                {isActive && runStatus === 'needs_input' && (
                  <span className="inline-block w-2 h-2 rounded-full bg-mode-ask animate-pulse" />
                )}
                <button
                  type="button"
                  aria-label={`删除 ${proj.title || 'Untitled'}`}
                  onClick={(e) => { e.stopPropagation(); handleRemove(proj); }}
                  className="p-0.5 rounded hover:bg-black/10 opacity-0 group-hover:opacity-100 transition-opacity"
                >
                  <X className="w-3 h-3" />
                </button>
              </div>
            );
          })}
          
          <button 
            onClick={handleCreate}
            className="h-10 px-3 ml-1 rounded-t-md text-text-600 hover:bg-black/5 hover:text-text-900 transition-colors flex items-center"
            title="Create new project"
          >
            <Plus className="w-4 h-4" />
          </button>
        </div>
      )}
    </div>
  );
};
