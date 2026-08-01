import React from 'react';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from '../../components/ui/dialog';
import { useProjectStore } from '../../stores/projectStore';
import { FilePlus, FolderOpen } from 'lucide-react';

interface ProjectPickerModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onOpenExisting: () => void;
}

export const ProjectPickerModal: React.FC<ProjectPickerModalProps> = ({ open, onOpenChange, onOpenExisting }) => {
  const { startPendingNewProject } = useProjectStore();

  const handleNewProject = () => {
    onOpenChange(false);
    startPendingNewProject();
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle>选择操作</DialogTitle>
          <DialogDescription className="sr-only">选择新建项目或打开已有项目</DialogDescription>
        </DialogHeader>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mt-4">
          <button
            onClick={handleNewProject}
            className="flex flex-col items-start p-6 text-left border border-border rounded-lg hover:border-accent hover:bg-accent-soft transition-colors group"
          >
            <div className="w-10 h-10 rounded-full bg-surface border border-border flex items-center justify-center mb-4 group-hover:border-accent/30 group-hover:text-accent transition-colors">
              <FilePlus className="w-5 h-5 text-text-600 group-hover:text-accent" />
            </div>
            <h3 className="text-base font-medium text-text-900 mb-1">新建项目</h3>
            <p className="text-sm text-text-400">从零开始一个新的演示文稿</p>
          </button>

          <button
            onClick={onOpenExisting}
            className="flex flex-col items-start p-6 text-left border border-border rounded-lg hover:border-accent hover:bg-accent-soft transition-colors group"
          >
            <div className="w-10 h-10 rounded-full bg-surface border border-border flex items-center justify-center mb-4 group-hover:border-accent/30 group-hover:text-accent transition-colors">
              <FolderOpen className="w-5 h-5 text-text-600 group-hover:text-accent" />
            </div>
            <h3 className="text-base font-medium text-text-900 mb-1">打开已有项目</h3>
            <p className="text-sm text-text-400">从最近的项目里继续</p>
          </button>
        </div>
      </DialogContent>
    </Dialog>
  );
};
