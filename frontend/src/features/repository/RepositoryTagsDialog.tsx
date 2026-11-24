import { useState } from 'react';
import { Plus, Trash2 } from 'lucide-react';
import { assetsApi, type AssetTag } from '../../api/assets';
import type { ResourceScope } from '../../api/types';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '../../components/ui/dialog';
import { IconButton } from '../../components/ui/primitives';
import { showGlobalError } from '../../stores/toastStore';

export function RepositoryTagsDialog({ type, tags, onClose, onChange }: {
  type: ResourceScope; tags: AssetTag[]; onClose: () => void; onChange: () => Promise<void>;
}) {
  const [name, setName] = useState('');
  const [busy, setBusy] = useState(false);
  const change = async (tag?: AssetTag) => {
    if (busy || (tag && !window.confirm(`删除标签「${tag.name}」？`))) return;
    setBusy(true);
    try {
      if (tag) await assetsApi.deleteTag(tag.id);
      else { await assetsApi.createTag(type, name); setName(''); }
      await onChange();
    } catch (cause) { showGlobalError(cause instanceof Error ? cause.message : '标签更新失败'); }
    finally { setBusy(false); }
  };
  return <Dialog open onOpenChange={open => { if (!open && !busy) onClose(); }}>
    <DialogContent><DialogHeader><DialogTitle>标签</DialogTitle></DialogHeader>
      <form className="flex gap-2" onSubmit={event => { event.preventDefault(); void change(); }}>
        <input aria-label="标签名称" value={name} onChange={event => setName(event.target.value)} className="h-9 flex-1 rounded-lg border border-border bg-panel px-3 text-sm" />
        <IconButton label="创建标签" type="submit" disabled={busy || !name.trim()}><Plus size={16} /></IconButton>
      </form>
      <div className="max-h-72 overflow-y-auto">{tags.map(tag => <div key={tag.id} className="flex items-center justify-between py-1 text-sm">
        {tag.name}<IconButton label={`删除标签${tag.name}`} disabled={busy} onClick={() => void change(tag)}><Trash2 size={16} /></IconButton>
      </div>)}</div>
    </DialogContent>
  </Dialog>;
}
