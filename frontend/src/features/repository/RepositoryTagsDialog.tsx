import { useState } from 'react';
import { Check, Pencil, Plus, Trash2, X } from 'lucide-react';
import { assetsApi, type AssetTag } from '../../api/assets';
import type { ResourceScope } from '../../api/types';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '../../components/ui/dialog';
import { IconButton } from '../../components/ui/primitives';
import { showGlobalError } from '../../stores/toastStore';

export function RepositoryTagsDialog({ type, tags, onClose, onChange }: {
  type: ResourceScope; tags: AssetTag[]; onClose: () => void; onChange: () => Promise<void>;
}) {
  const [name, setName] = useState('');
  const [editing, setEditing] = useState<AssetTag>();
  const [busy, setBusy] = useState(false);
  const change = async (write: () => Promise<void>) => {
    if (busy) return;
    setBusy(true);
    try {
      await write();
      await onChange();
    } catch (cause) { showGlobalError(cause instanceof Error ? cause.message : '标签更新失败'); }
    finally { setBusy(false); }
  };
  return <Dialog open onOpenChange={open => { if (!open && !busy) onClose(); }}>
    <DialogContent><DialogHeader><DialogTitle>标签</DialogTitle></DialogHeader>
      <form className="flex gap-2" onSubmit={event => {
        event.preventDefault();
        void change(async () => { await assetsApi.createTag(type, name); setName(''); });
      }}>
        <input aria-label="标签名称" value={name} onChange={event => setName(event.target.value)} className="h-9 flex-1 rounded-lg border border-border bg-panel px-3 text-sm" />
        <IconButton label="创建标签" type="submit" disabled={busy || !name.trim()}><Plus size={16} /></IconButton>
      </form>
      <div className="max-h-72 overflow-y-auto">{tags.map(tag => editing?.id === tag.id ? (
        <form key={tag.id} className="flex items-center gap-1 py-1" onSubmit={event => {
          event.preventDefault();
          void change(async () => { await assetsApi.updateTag(tag.id, editing.name); setEditing(undefined); });
        }}>
          <input autoFocus aria-label="编辑标签名称" value={editing.name} disabled={busy}
            onChange={event => setEditing({ id: tag.id, name: event.target.value })}
            className="h-9 min-w-0 flex-1 rounded-lg border border-border bg-panel px-3 text-sm" />
          <IconButton label="保存标签名称" type="submit" disabled={busy || !editing.name.trim()}><Check size={16} /></IconButton>
          <IconButton label="取消编辑" type="button" disabled={busy} onClick={() => setEditing(undefined)}><X size={16} /></IconButton>
        </form>
      ) : (
        <div key={tag.id} className="flex items-center gap-1 py-1 text-sm">
          <span className="min-w-0 flex-1 truncate">{tag.name}</span>
          <IconButton label={`编辑标签${tag.name}`} disabled={busy} onClick={() => setEditing(tag)}><Pencil size={16} /></IconButton>
          <IconButton label={`删除标签${tag.name}`} disabled={busy} onClick={() => {
            if (window.confirm(`删除标签「${tag.name}」？`)) void change(() => assetsApi.deleteTag(tag.id));
          }}><Trash2 size={16} /></IconButton>
        </div>
      ))}</div>
    </DialogContent>
  </Dialog>;
}
