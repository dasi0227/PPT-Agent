import { useEffect, useState } from 'react';
import { Button } from '../../components/ui/primitives';
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '../../components/ui/dialog';
import { cn } from '../../lib/utils';

export interface RepositoryMetadataDraft<T extends string> {
  name: string;
  description: string;
  tags: T[];
}

export function RepositoryEditDialog<T extends string>({
  open,
  title,
  value,
  tagOptions,
  onOpenChange,
  onSave,
}: {
  open: boolean;
  title: string;
  value: RepositoryMetadataDraft<T>;
  tagOptions: Array<{ value: T; label: string }>;
  onOpenChange: (open: boolean) => void;
  onSave: (value: RepositoryMetadataDraft<T>) => Promise<void>;
}) {
  const [draft, setDraft] = useState(value);
  const [saving, setSaving] = useState(false);
  const valueTags = value.tags.join('\u0000');

  useEffect(() => {
    if (open) {
      setDraft({
        name: value.name,
        description: value.description,
        tags: valueTags ? valueTags.split('\u0000') as T[] : [],
      });
    }
  }, [open, value.description, value.name, valueTags]);

  const dirty = draft.name !== value.name
    || draft.description !== value.description
    || draft.tags.join('\u0000') !== valueTags;

  const requestOpenChange = (next: boolean) => {
    if (saving) return;
    if (!next && dirty && !window.confirm('当前修改尚未保存，确定放弃吗？')) return;
    onOpenChange(next);
  };

  const valid = draft.name.trim().length > 0
    && draft.name.trim().length <= 80
    && draft.description.trim().length > 0
    && draft.description.trim().length <= 500
    && draft.tags.length <= 2;

  const toggleTag = (tag: T) => {
    setDraft((current) => ({
      ...current,
      tags: current.tags.includes(tag)
        ? current.tags.filter((value) => value !== tag)
        : current.tags.length < 2 ? [...current.tags, tag] : current.tags,
    }));
  };

  const submit = async () => {
    if (!valid || saving) return;
    setSaving(true);
    try {
      await onSave({
        name: draft.name.trim(),
        description: draft.description.trim(),
        tags: draft.tags,
      });
      onOpenChange(false);
    } finally {
      setSaving(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={requestOpenChange}>
      <DialogContent className="gap-0 p-0 sm:max-w-[480px]">
        <DialogHeader className="border-b border-border px-5 py-4">
          <DialogTitle>{title}</DialogTitle>
        </DialogHeader>
        <form
          className="space-y-4 px-5 py-5"
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
              className="h-9 rounded-lg border border-border bg-surface px-3 text-[13px] text-text-900 outline-none focus:border-accent focus:ring-2 focus:ring-accent/10"
            />
          </label>
          <label className="grid gap-1.5 text-xs font-semibold text-text-600">
            描述
            <textarea
              maxLength={500}
              rows={4}
              value={draft.description}
              onChange={(event) => setDraft({ ...draft, description: event.target.value })}
              className="resize-none rounded-lg border border-border bg-surface px-3 py-2 text-[13px] leading-5 text-text-900 outline-none focus:border-accent focus:ring-2 focus:ring-accent/10"
            />
          </label>
          <fieldset className="grid gap-1.5">
            <legend className="mb-1.5 text-xs font-semibold text-text-600">标签</legend>
            <div className="flex flex-wrap gap-1.5">
              {tagOptions.map((option) => {
                const selected = draft.tags.includes(option.value);
                return (
                  <button
                    key={option.value}
                    type="button"
                    aria-pressed={selected}
                    disabled={!selected && draft.tags.length >= 2}
                    onClick={() => toggleTag(option.value)}
                    className={cn(
                      'h-7 rounded-md border px-2.5 text-xs font-semibold focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent disabled:cursor-not-allowed disabled:opacity-40',
                      selected ? 'border-accent/30 bg-accent-soft text-accent' : 'border-border bg-surface text-text-600 hover:bg-panel-muted',
                    )}
                  >
                    {option.label}
                  </button>
                );
              })}
            </div>
          </fieldset>
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
