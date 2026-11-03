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
        description={<>此操作会<strong className="font-semibold text-danger underline decoration-danger/70 decoration-2 underline-offset-4">永久删除</strong>该会话中的<strong className="font-semibold text-danger underline decoration-danger/70 decoration-2 underline-offset-4">所有内容</strong>，且<strong className="font-semibold text-danger underline decoration-danger/70 decoration-2 underline-offset-4">无法恢复</strong>。<br />请慎重考虑。</>}
        confirmLabel="删除"
        variant="danger"
        onConfirm={async () => {
          await deleteThread(projectId, thread.id);
        }}
      />
    </>
  );
};
