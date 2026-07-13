import React, { useEffect } from 'react';
import { PickerModal } from '../../components/ui/modal-picker';
import { useProjectStore } from '../../stores/projectStore';
import { Project } from '../../api/types';

interface OpenExistingProjectModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export const OpenExistingProjectModal: React.FC<OpenExistingProjectModalProps> = ({ open, onOpenChange }) => {
  const { projects, openProject, loadProjects } = useProjectStore();

  useEffect(() => {
    if (open) {
      loadProjects();
    }
  }, [open, loadProjects]);

  const handlePick = (project: Project) => {
    openProject(project.id);
    onOpenChange(false);
  };

  const sortedProjects = [...projects].sort((a, b) => b.updated_at - a.updated_at);

  const formatRelativeTime = (timestamp: number) => {
    const rtf = new Intl.RelativeTimeFormat('zh-CN', { numeric: 'auto' });
    const daysDifference = Math.round((timestamp * 1000 - Date.now()) / (1000 * 60 * 60 * 24));
    if (daysDifference === 0) return '今天';
    return rtf.format(daysDifference, 'day');
  };

  return (
    <PickerModal<Project>
      open={open}
      onOpenChange={onOpenChange}
      title="打开已有项目"
      items={sortedProjects}
      keyOf={(p) => p.id}
      searchOf={(p) => p.title}
      onPick={handlePick}
      emptyState={
        <div className="p-8 text-center flex flex-col items-center justify-center text-text-400">
          <p className="mb-4">还没有历史项目，去新建一个</p>
          <button
            onClick={() => onOpenChange(false)}
            className="px-4 py-2 text-sm font-medium rounded-md border border-border hover:bg-black/5 transition-colors"
          >
            返回
          </button>
        </div>
      }
      renderItem={(p) => (
        <div className="flex flex-col">
          <span className="font-medium text-text-900 text-sm mb-1">{p.title || 'Untitled Project'}</span>
          <div className="flex items-center gap-3 text-xs text-text-400">
            <span>更新于 {formatRelativeTime(p.updated_at)}</span>
          </div>
        </div>
      )}
    />
  );
};
