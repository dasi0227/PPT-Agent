import React, { useState } from 'react';
import { DropdownMenu, DropdownMenuTrigger, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator } from '../../components/ui/dropdown-menu';
import { FormModal } from '../../components/ui/modal-form';
import { ConfirmModal } from '../../components/ui/modal-confirm';
import { useProjectStore } from '../../stores/projectStore';
import { Project } from '../../api/types';

interface ProjectMenuProps {
  project: Pick<Project, 'id' | 'title'>;
  children: React.ReactNode;
}

export const ProjectMenu: React.FC<ProjectMenuProps> = ({ project, children }) => {
  const { renameProject, closeProject, deleteProject } = useProjectStore();
  
  const [renameOpen, setRenameOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          {children}
        </DropdownMenuTrigger>
        <DropdownMenuContent side="bottom" align="end" sideOffset={4}>
          <DropdownMenuItem onSelect={() => setRenameOpen(true)}>
            重命名
          </DropdownMenuItem>
          <DropdownMenuItem onSelect={() => closeProject(project.id)}>
            关闭项目
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem destructive onSelect={() => setDeleteOpen(true)}>
            删除项目
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <FormModal<string>
        open={renameOpen}
        onOpenChange={setRenameOpen}
        title="重命名项目"
        initialValue={project.title || ''}
        validate={(val) => {
          const t = val.trim();
          if (t.length < 1) return '名称不能为空';
          if (t.length > 60) return '名称不能超过 60 个字符';
          return null;
        }}
        onSubmit={async (val) => {
          await renameProject(project.id, val.trim());
        }}
        renderField={(val, setVal) => (
          <div>
            <input
              autoFocus
              type="text"
              value={val}
              onChange={(e) => setVal(e.target.value)}
              className="w-full px-3 py-2 bg-background border border-border rounded-md text-sm focus:outline-none focus:border-mode-normal focus:ring-1 focus:ring-mode-normal transition-all"
              placeholder="请输入项目名称"
            />
          </div>
        )}
      />

      <ConfirmModal
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title={`删除项目「${project.title || 'Untitled'}」？`}
        description="此操作不可撤销，项目的所有会话、切片、历史记录都会被移除。"
        confirmLabel="删除"
        variant="danger"
        onConfirm={async () => {
          await deleteProject(project.id);
        }}
      />
    </>
  );
};
