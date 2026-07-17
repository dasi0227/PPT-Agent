import React from 'react';
import { useProjectStore } from '../../stores/projectStore';
import { useActiveSession } from '../agent/useActiveSession';
import { cn } from '../../lib/utils';
import { Loader2, Plus, MoreHorizontal } from 'lucide-react';
import { ProjectMenu } from './ProjectMenu';

export const ProjectTabs: React.FC = () => {
  const { projects, activeProjectId, selectProject, loadingProjects, loadProjects } = useProjectStore();
  const { status: runStatus } = useActiveSession();

  React.useEffect(() => {
    loadProjects();
  }, [loadProjects]);

  const openProjectIds = useProjectStore((s) => s.openProjectIds);
  
  // Combine open projects and pending
  const displayProjects = openProjectIds.map(id => {
    if (id === 'new-pending') {
      return { id, title: '新建中…' };
    }
    return projects.find(p => p.id === id) || { id, title: 'Loading...' };
  });

  return (
    <div className="flex items-center h-12 bg-background border-b border-border-strong px-2 overflow-x-auto select-none">
      <div className="flex-shrink-0 mr-4 font-bold text-text-900 px-2 flex items-center">
        <img src="/logo.jpg" alt="Logo" className="w-5 h-5 rounded-sm mr-2 object-cover" />
        M7 Studio
      </div>

      {loadingProjects && displayProjects.length === 0 ? (
        <Loader2 className="w-4 h-4 animate-spin text-text-400" />
      ) : (
        <div className="flex items-end h-full gap-1 flex-1">
          {displayProjects.map((proj) => {
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
                
                {isActive && runStatus === 'running' && (
                  <span className="inline-block w-2 h-2 rounded-full bg-mode-normal animate-pulse" />
                )}
                {isActive && runStatus === 'needs_input' && (
                  <span className="inline-block w-2 h-2 rounded-full bg-mode-ask animate-pulse" />
                )}
                
                {proj.id !== 'new-pending' && (
                  <ProjectMenu project={proj}>
                    <button
                      type="button"
                      aria-label={`更多选项`}
                      onClick={(e) => e.stopPropagation()}
                      className="p-0.5 rounded hover:bg-black/10 opacity-0 group-hover:opacity-100 transition-opacity"
                    >
                      <MoreHorizontal className="w-4 h-4 text-text-400" />
                    </button>
                  </ProjectMenu>
                )}
              </div>
            );
          })}

          <button
            onClick={() => {
              // trigger ProjectPickerModal (M5)
              document.dispatchEvent(new CustomEvent('open-project-picker'));
            }}
            className="h-10 px-3 ml-1 rounded-t-md text-text-600 hover:bg-black/5 hover:text-text-900 transition-colors flex items-center"
            title="新建或打开项目"
          >
            <Plus className="w-4 h-4" />
          </button>
        </div>
      )}
    </div>
  );
};
