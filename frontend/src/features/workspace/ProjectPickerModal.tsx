import React from 'react';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle, DialogDescription } from '../../components/ui/dialog';
import { useProjectStore } from '../../stores/projectStore';
import { ArrowLeft, FilePlus, FolderOpen, Loader2 } from 'lucide-react';

interface ProjectPickerModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onOpenExisting: () => void;
}

export const ProjectPickerModal: React.FC<ProjectPickerModalProps> = ({ open, onOpenChange, onOpenExisting }) => {
  const { createProject, openProject } = useProjectStore();
  const [mode, setMode] = React.useState<'choose' | 'create'>('choose');
  const [title, setTitle] = React.useState('');
  const [isCreating, setIsCreating] = React.useState(false);
  const [createError, setCreateError] = React.useState('');

  const reset = () => {
    setMode('choose');
    setTitle('');
    setCreateError('');
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
    setCreateError('');
    try {
      const project = await createProject(projectTitle, '', 10, 'zh-CN');
      openProject(project.id);
      reset();
      onOpenChange(false);
    } catch (error) {
      setCreateError(error instanceof Error ? error.message : '创建项目失败，请重试');
      setIsCreating(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle>{mode === 'choose' ? '选择操作' : '新建项目'}</DialogTitle>
          <DialogDescription>
            {mode === 'choose' ? '新建一个演示文稿，或继续已有项目。' : '为这个演示文稿输入一个便于识别的名称。'}
          </DialogDescription>
        </DialogHeader>
        {mode === 'choose' ? (
          <div className="mt-4 grid grid-cols-1 gap-3 md:grid-cols-2">
            <button
              type="button"
              onClick={() => setMode('create')}
              className="group flex flex-col items-start rounded-lg border border-border p-5 text-left transition-colors hover:border-accent hover:bg-accent-soft focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
            >
              <div className="mb-4 flex h-10 w-10 items-center justify-center rounded-md border border-border bg-surface transition-colors group-hover:border-accent/30 group-hover:text-accent">
                <FilePlus className="h-5 w-5 text-text-600 group-hover:text-accent" strokeWidth={1.75} />
              </div>
              <h3 className="mb-1 text-base font-medium text-text-900">新建项目</h3>
              <p className="text-sm text-text-400">命名后创建一个新的演示文稿。</p>
            </button>

            <button
              type="button"
              onClick={() => {
                reset();
                onOpenExisting();
              }}
              className="group flex flex-col items-start rounded-lg border border-border p-5 text-left transition-colors hover:border-accent hover:bg-accent-soft focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
            >
              <div className="mb-4 flex h-10 w-10 items-center justify-center rounded-md border border-border bg-surface transition-colors group-hover:border-accent/30 group-hover:text-accent">
                <FolderOpen className="h-5 w-5 text-text-600 group-hover:text-accent" strokeWidth={1.75} />
              </div>
              <h3 className="mb-1 text-base font-medium text-text-900">打开已有项目</h3>
              <p className="text-sm text-text-400">从最近的项目中继续工作。</p>
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
              <p className="text-xs text-text-600">创建后可随时在项目菜单中重命名。</p>
              {createError && <p role="alert" className="text-xs text-danger">{createError}</p>}
            </div>
            <DialogFooter>
              <button
                type="button"
                onClick={() => { setCreateError(''); setMode('choose'); }}
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
