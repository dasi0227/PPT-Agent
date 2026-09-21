import React, { useState } from 'react';
import { DropdownMenu, DropdownMenuTrigger, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator } from '../../components/ui/dropdown-menu';
import { ConfirmModal } from '../../components/ui/modal-confirm';
import { useThreadStore } from '../../stores/threadStore';
import { Thread } from '../../api/types';

interface ThreadMenuProps {
  projectId: string;
  thread: Pick<Thread, 'id' | 'title'>;
  children: React.ReactNode;
}

export const ThreadMenu: React.FC<ThreadMenuProps> = ({ projectId, thread, children }) => {
  const { openRenamePanel, deleteThread } = useThreadStore();
  
  const [deleteOpen, setDeleteOpen] = useState(false);

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          {children}
        </DropdownMenuTrigger>
        <DropdownMenuContent side="bottom" align="end" sideOffset={4}>
          <DropdownMenuItem onSelect={() => openRenamePanel(projectId, thread.id)}>
            命名设置
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem destructive onSelect={() => setDeleteOpen(true)}>
            删除会话
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <ConfirmModal
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title={`删除会话「${thread.title || '新会话'}」？`}
        description="此操作不可撤销，历史记录将永久丢失。"
        confirmLabel="删除"
        variant="danger"
        onConfirm={async () => {
          await deleteThread(projectId, thread.id);
        }}
      />
    </>
  );
};
