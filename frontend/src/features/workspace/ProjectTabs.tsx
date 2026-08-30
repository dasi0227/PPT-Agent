import React from 'react';
import { useNavigate } from 'react-router-dom';
import { useProjectStore } from '../../stores/projectStore';
import { useActiveSession } from '../agent/useActiveSession';
import { cn } from '../../lib/utils';
import { BookOpenText, Component, Loader2, Palette, Plus, MoreHorizontal, Warehouse } from 'lucide-react';
import { ProjectMenu } from './ProjectMenu';
import { projectRoute, repositoryRoute } from './routes';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from '../../components/ui/dropdown-menu';

export const ProjectTabs: React.FC = () => {
  const { projects, activeProjectId, selectProject, loadingProjects, loadProjects } = useProjectStore();
  const { status: runStatus } = useActiveSession();
  const navigate = useNavigate();

  React.useEffect(() => {
    loadProjects();
  }, [loadProjects]);

  const openProjectIds = useProjectStore((s) => s.openProjectIds);
  
  const displayProjects = openProjectIds.map((id) => projects.find((project) => project.id === id) || { id, title: '加载中…' });
  const activateProject = (projectId: string) => {
    selectProject(projectId);
    navigate(projectRoute(projectId));
  };

  return (
    <div className="flex h-12 items-center overflow-hidden border-b border-border-strong bg-panel px-2 select-none">
      <div className="mr-4 flex shrink-0 items-center px-2 font-bold text-text-900">
        <img src="/logo.jpg" alt="Logo" className="w-10 h-10 rounded-sm mr-2 object-cover" />
        Dasi PPT Agent
      </div>

      <div className="flex min-w-0 flex-1 items-center">
        {loadingProjects && displayProjects.length === 0 ? (
          <Loader2 className="h-4 w-4 animate-spin text-text-400" />
        ) : (
          <div className="flex h-full min-w-0 flex-1 items-end gap-1 overflow-x-auto">
          {displayProjects.map((proj) => {
            const isActive = proj.id === activeProjectId;
            return (
              <div
                key={proj.id}
                role="tab"
                aria-selected={isActive}
                tabIndex={0}
                onClick={() => activateProject(proj.id)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter' || event.key === ' ') {
                    event.preventDefault();
                    activateProject(proj.id);
                  }
                }}
                className={cn(
                  "group h-10 px-3 pl-4 rounded-t-md text-sm font-medium transition-colors border border-b-0 flex items-center gap-1 cursor-pointer focus-visible:ring-inset",
                  isActive
                    ? "bg-panel text-text-900 border-border-strong border-b-panel relative top-[1px]"
                    : "bg-transparent text-text-600 border-transparent hover:bg-panel-muted"
                )}
              >
                <span className="truncate max-w-[160px]">{proj.title || '未命名项目'}</span>
                
                {isActive && runStatus === 'running' && (
                  <span aria-label="运行中" className="inline-block w-2 h-2 rounded-full bg-accent animate-pulse" />
                )}
                {isActive && runStatus === 'waiting' && (
                  <span aria-label="等待输入" className="inline-block w-2 h-2 rounded-full bg-warning animate-pulse" />
                )}
                {isActive && runStatus === 'paused' && (
                  <span aria-label="已暂停" className="inline-block h-2 w-2 rounded-full bg-text-400" />
                )}
                
                <ProjectMenu project={proj}>
                  <button
                    type="button"
                    aria-label="更多选项"
                    onClick={(e) => e.stopPropagation()}
                    className="rounded p-0.5 opacity-0 transition-opacity hover:bg-black/10 group-hover:opacity-100 group-focus-within:opacity-100"
                  >
                    <MoreHorizontal className="h-4 w-4 text-text-400" />
                  </button>
                </ProjectMenu>
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
            aria-label="新建或打开项目"
          >
            <Plus className="w-4 h-4" />
          </button>
          </div>
        )}
      </div>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button type="button" className="ml-2 grid h-8 w-8 shrink-0 place-items-center rounded-md text-text-600 hover:bg-panel-muted hover:text-text-900" title="个人仓库" aria-label="个人仓库">
            <Warehouse className="h-4 w-4" />
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-64 p-1">
          <DropdownMenuLabel>个人仓库</DropdownMenuLabel>
          {[
            { id: 'theme' as const, label: '主题', description: '选择演示文稿的整体样式', Icon: Palette },
            { id: 'component' as const, label: '组件', description: '浏览供 Agent 参考的片段', Icon: Component },
            { id: 'skill' as const, label: '技能', description: '管理 Run 可使用的技能', Icon: BookOpenText },
          ].map(({ id, label, description, Icon }) => (
            <DropdownMenuItem key={id} onSelect={() => navigate(repositoryRoute(id))} className="flex min-h-12 items-start gap-2 rounded-md px-2 py-2">
              <Icon className="mt-0.5 h-4 w-4 shrink-0 text-text-500" />
              <span><b className="block text-xs text-text-900">{label}</b><small className="block text-[11px] text-text-500">{description}</small></span>
            </DropdownMenuItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
};
