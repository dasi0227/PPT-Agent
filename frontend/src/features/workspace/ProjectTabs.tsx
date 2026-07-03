import React, { useEffect } from 'react';
import { useProjectStore } from '../../stores/projectStore';
import { useRunStore } from '../../stores/runStore';
import { cn } from '../../lib/utils';
import { Loader2, Plus } from 'lucide-react';

export const ProjectTabs: React.FC = () => {
  const { projects, activeProjectId, selectProject, loadingProjects, loadProjects, createProject } = useProjectStore();
  const { status: runStatus } = useRunStore();

  useEffect(() => {
    loadProjects();
  }, [loadProjects]);

  const handleCreate = async () => {
    await createProject('New Presentation', 'default');
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
              <button
                key={proj.id}
                onClick={() => selectProject(proj.id)}
                className={cn(
                  "h-10 px-4 rounded-t-md text-sm font-medium transition-colors border border-b-0",
                  isActive 
                    ? "bg-surface text-text-900 border-border-strong border-b-transparent shadow-sm relative top-[1px]" 
                    : "bg-background text-text-600 border-transparent hover:bg-black/5"
                )}
              >
                {proj.title}
                {isActive && runStatus === 'running' && (
                  <span className="ml-2 inline-block w-2 h-2 rounded-full bg-mode-normal animate-pulse" />
                )}
                {isActive && runStatus === 'needs_input' && (
                  <span className="ml-2 inline-block w-2 h-2 rounded-full bg-mode-ask animate-pulse" />
                )}
              </button>
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
