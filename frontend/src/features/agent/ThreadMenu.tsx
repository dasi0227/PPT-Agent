import React, { useState } from 'react';
import { DropdownMenu, DropdownMenuTrigger, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator } from '../../components/ui/dropdown-menu';
import { FormModal } from '../../components/ui/modal-form';
import { ConfirmModal } from '../../components/ui/modal-confirm';
import { useThreadStore } from '../../stores/threadStore';
import { Thread } from '../../api/types';

interface ThreadMenuProps {
  projectId: string;
  thread: Pick<Thread, 'id' | 'title'>;
  children: React.ReactNode;
}

export const ThreadMenu: React.FC<ThreadMenuProps> = ({ projectId, thread, children }) => {
  const { renameThread, deleteThread } = useThreadStore();
  
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
          <DropdownMenuSeparator />
          <DropdownMenuItem destructive onSelect={() => setDeleteOpen(true)}>
            删除会话
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <FormModal<string>
        open={renameOpen}
        onOpenChange={setRenameOpen}
        title="重命名会话"
        initialValue={thread.title || ''}
        validate={(val) => {
          const t = val.trim();
          if (t.length < 1) return '名称不能为空';
          if (t.length > 60) return '名称不能超过 60 个字符';
          return null;
        }}
        onSubmit={async (val) => {
          await renameThread(projectId, thread.id, val.trim());
        }}
        renderField={(val, setVal) => (
          <div>
            <input
              autoFocus
              type="text"
              value={val}
              onChange={(e) => setVal(e.target.value)}
              className="w-full px-3 py-2 bg-panel border border-border rounded-md text-sm focus:border-accent transition-colors"
              placeholder="请输入会话名称"
            />
          </div>
        )}
      />

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
