import React, { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { PickerModal } from '../../components/ui/modal-picker';
import { useProjectStore } from '../../stores/projectStore';
import { Project } from '../../api/types';
import { ArrowLeft } from 'lucide-react';
import { projectRoute } from './routes';

interface OpenExistingProjectModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onBack: () => void;
}

export const OpenExistingProjectModal: React.FC<OpenExistingProjectModalProps> = ({ open, onOpenChange, onBack }) => {
  const { projects, openProject, loadProjects } = useProjectStore();
  const navigate = useNavigate();

  useEffect(() => {
    if (open) {
      loadProjects();
    }
  }, [open, loadProjects]);

  const handlePick = (project: Project) => {
    openProject(project.id);
    navigate(projectRoute(project.id));
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
            onClick={onBack}
            className="inline-flex h-9 items-center justify-center gap-1 rounded-md border border-border px-3 text-sm font-medium text-text-600 transition-colors hover:bg-panel-muted hover:text-text-900"
          >
            <ArrowLeft className="h-4 w-4" strokeWidth={1.75} /> 返回
          </button>
        </div>
      }
      renderItem={(p) => (
        <div className="flex flex-col">
          <span className="font-medium text-text-900 text-sm mb-1">{p.title || '未命名项目'}</span>
          <div className="flex items-center gap-3 text-xs text-text-400">
            <span>更新于 {formatRelativeTime(p.updated_at)}</span>
          </div>
        </div>
      )}
    />
  );
};
