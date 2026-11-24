import type { AssetDraft, AssetTag } from '../../api/assets';
import { showGlobalError } from '../../stores/toastStore';
import { useState } from 'react';
import { Button } from '../../components/ui/primitives';
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '../../components/ui/dialog';
import { cn } from '../../lib/utils';

export function RepositoryEditDialog({ title, value, tags, onClose, onSave }: {
  title: string;
  value: AssetDraft;
  tags: AssetTag[];
  onClose: () => void;
  onSave: (draft: AssetDraft) => Promise<void>;
}) {
  const [draft, setDraft] = useState(value);
  const [saving, setSaving] = useState(false);
  const dirty = JSON.stringify(draft) !== JSON.stringify(value);
  const requestOpenChange = (next: boolean) => {
    if (saving) return;
    if (!next && dirty && !window.confirm('当前修改尚未保存，确定放弃吗？')) return;
    if (!next) onClose();
  };

  const valid = draft.name.trim().length > 0
    && draft.name.trim().length <= 80
    && draft.description.trim().length > 0
    && draft.description.trim().length <= 500
    && draft.content.length > 0;

  const toggleTag = (tag: string) => {
    setDraft((current) => ({
      ...current,
      tag_ids: current.tag_ids.includes(tag)
        ? current.tag_ids.filter((value) => value !== tag)
        : [...current.tag_ids, tag],
    }));
  };

  const submit = async () => {
    if (!valid || saving) return;
    setSaving(true);
    try {
      await onSave(draft);
      onClose();
    } catch (cause) {
      showGlobalError(cause instanceof Error ? cause.message : '保存失败');
    } finally {
      setSaving(false);
    }
  };

  return (
    <Dialog open onOpenChange={requestOpenChange}>
      <DialogContent className="gap-0 p-0 sm:max-w-[480px]">
        <DialogHeader className="border-b border-border px-5 py-4">
          <DialogTitle>{title}</DialogTitle>
        </DialogHeader>
        <form
          className="max-h-[75dvh] space-y-4 overflow-y-auto px-5 py-5"
          onSubmit={(event) => {
            event.preventDefault();
            void submit();
          }}
        >
          <label className="grid gap-1.5 text-xs font-semibold text-text-600">
            名称
            <input
              autoFocus
              maxLength={80}
              value={draft.name}
              onChange={(event) => setDraft({ ...draft, name: event.target.value })}
              className="h-9 rounded-lg border border-border bg-surface px-3 text-[13px] text-text-900 outline-none"
            />
          </label>
          <fieldset className="grid gap-1.5">
            <legend className="mb-1.5 text-xs font-semibold text-text-600">标签</legend>
            <div className="flex flex-wrap gap-1.5">
              {tags.map((option) => {
                const selected = draft.tag_ids.includes(option.id);
                return (
                  <button
                    key={option.id}
                    type="button"
                    aria-pressed={selected}
                    onClick={() => toggleTag(option.id)}
                    className={cn(
                      'h-7 rounded-md border px-2.5 text-xs font-medium focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-40',
                      selected ? 'border-accent/30 ui-selected' : 'border-border bg-surface text-text-600 ui-interactive',
                    )}
                  >
                    {option.name}
                  </button>
                );
              })}
            </div>
          </fieldset>
          <label className="grid gap-1.5 text-xs font-semibold text-text-600">
            描述
            <textarea
              maxLength={500}
              rows={4}
              value={draft.description}
              onChange={(event) => setDraft({ ...draft, description: event.target.value })}
              className="resize-none rounded-lg border border-border bg-surface px-3 py-2 text-[13px] leading-5 text-text-900 outline-none"
            />
          </label>
          <label className="grid gap-1.5 text-xs font-semibold text-text-600">正文
            <textarea rows={5} value={draft.content} onChange={event => setDraft({ ...draft, content: event.target.value })}
              className="rounded-lg border border-border bg-surface px-3 py-2 font-mono text-[13px] text-text-900" />
          </label>
          <DialogFooter className="pt-2">
            <Button type="button" variant="secondary" disabled={saving} onClick={() => requestOpenChange(false)}>
              取消
            </Button>
            <Button type="submit" variant="primary" disabled={!valid || saving}>
              {saving ? '保存中' : '保存'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
