import React from 'react';
import { useNavigate } from 'react-router-dom';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '../../components/ui/dialog';
import { useProjectStore } from '../../stores/projectStore';
import { ArrowLeft, FilePlus, FolderOpen, Loader2 } from 'lucide-react';
import { projectRoute } from './routes';

interface ProjectPickerModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onOpenExisting: () => void;
}

export const ProjectPickerModal: React.FC<ProjectPickerModalProps> = ({ open, onOpenChange, onOpenExisting }) => {
  const { createProject, openProject } = useProjectStore();
  const navigate = useNavigate();
  const [mode, setMode] = React.useState<'choose' | 'create'>('choose');
  const [title, setTitle] = React.useState('');
  const [isCreating, setIsCreating] = React.useState(false);

  const reset = () => {
    setMode('choose');
    setTitle('');
    setIsCreating(false);
  };

  const handleOpenChange = (nextOpen: boolean) => {
    if (!nextOpen && !isCreating) reset();
    onOpenChange(nextOpen);
  };

  const handleCreateProject = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const projectTitle = title.trim();
    if (!projectTitle || isCreating) return;

    setIsCreating(true);
    try {
      const project = await createProject(projectTitle, '', 10, 'zh-CN');
      openProject(project.id);
      navigate(projectRoute(project.id));
      reset();
      onOpenChange(false);
    } catch {
      // The API client reports non-Agent backend errors through the global toast layer.
      setIsCreating(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-[500px]">
        {mode === 'choose' ? (
          <DialogTitle className="sr-only">项目操作</DialogTitle>
        ) : (
          <DialogHeader>
            <DialogTitle>创建全新项目</DialogTitle>
          </DialogHeader>
        )}
        {mode === 'choose' ? (
          <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
            <button
              type="button"
              onClick={() => setMode('create')}
              className="group flex min-h-[210px] flex-col items-start justify-center rounded-lg border border-border p-6 text-left transition-colors hover:border-accent hover:bg-accent-soft focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
            >
              <div className="mb-5 flex h-14 w-14 items-center justify-center rounded-lg border border-border bg-surface transition-colors group-hover:border-accent/30 group-hover:text-accent">
                <FilePlus className="h-7 w-7 text-text-600 group-hover:text-accent" strokeWidth={1.75} />
              </div>
              <h3 className="text-lg font-semibold text-text-900">创建全新项目</h3>
            </button>

            <button
              type="button"
              onClick={() => {
                reset();
                onOpenExisting();
              }}
              className="group flex min-h-[210px] flex-col items-start justify-center rounded-lg border border-border p-6 text-left transition-colors hover:border-accent hover:bg-accent-soft focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
            >
              <div className="mb-5 flex h-14 w-14 items-center justify-center rounded-lg border border-border bg-surface transition-colors group-hover:border-accent/30 group-hover:text-accent">
                <FolderOpen className="h-7 w-7 text-text-600 group-hover:text-accent" strokeWidth={1.75} />
              </div>
              <h3 className="text-lg font-semibold text-text-900">打开已有项目</h3>
            </button>
          </div>
        ) : (
          <form className="mt-4 grid gap-5" onSubmit={handleCreateProject}>
            <div className="grid gap-2">
              <label htmlFor="new-project-title" className="text-sm font-medium text-text-900">项目名称</label>
              <input
                id="new-project-title"
                autoFocus
                value={title}
                onChange={(event) => setTitle(event.target.value)}
                disabled={isCreating}
                placeholder="例如：2026 品牌发布会"
                className="h-10 w-full rounded-md border border-border bg-panel px-3 text-sm text-text-900 placeholder:text-text-400 transition-colors focus:border-accent focus:outline-none focus:ring-2 focus:ring-accent/20 disabled:cursor-not-allowed disabled:opacity-60"
              />
            </div>
            <DialogFooter>
              <button
                type="button"
                onClick={() => setMode('choose')}
                disabled={isCreating}
                className="inline-flex h-9 items-center justify-center gap-1 rounded-md border border-border px-3 text-sm font-medium text-text-600 transition-colors hover:bg-panel-muted hover:text-text-900 disabled:cursor-not-allowed disabled:opacity-50"
              >
                <ArrowLeft className="h-4 w-4" strokeWidth={1.75} /> 返回
              </button>
              <button
                type="submit"
                disabled={!title.trim() || isCreating}
                className="inline-flex h-9 items-center justify-center gap-2 rounded-md bg-accent px-4 text-sm font-medium text-white transition-colors hover:bg-accent/90 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {isCreating && <Loader2 className="h-4 w-4 animate-spin" strokeWidth={1.75} />}
                {isCreating ? '正在创建' : '创建项目'}
              </button>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  );
};
