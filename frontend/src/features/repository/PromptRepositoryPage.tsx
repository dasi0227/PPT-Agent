import { Check, NotebookText, Pause, Plus, X } from 'lucide-react';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { APIError } from '../../api/client';
import type { Prompt, PromptTag, PromptWriteRequest } from '../../api/types';
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '../../components/ui/dialog';
import { Button } from '../../components/ui/primitives';
import { cn } from '../../lib/utils';
import { usePromptStore } from '../../stores/promptStore';
import { showGlobalError } from '../../stores/toastStore';
import { promptTagLabels, promptTagOrder } from '../agent/promptMatching';
import {
  RepositoryCatalog,
  RepositoryDetail,
  RepositoryDirectoryItem,
  RepositoryFilterButton,
  RepositoryLoading,
  RepositoryPageHeader,
  RepositoryState,
  RepositoryWorkspace,
} from './RepositoryPrimitives';
import { RepositoryShell } from './RepositoryShell';

const emptyDraft: PromptWriteRequest = { name: '', desc: '', value: '', tags: [] };

type FieldErrors = Partial<Record<keyof PromptWriteRequest, string>>;

function promptDraft(prompt: Prompt): PromptWriteRequest {
  return { name: prompt.name, desc: prompt.desc, value: prompt.value, tags: [...prompt.tags] };
}

function sameDraft(left: PromptWriteRequest, right: PromptWriteRequest): boolean {
  return left.name === right.name && left.desc === right.desc && left.value === right.value
    && left.tags.join('|') === right.tags.join('|');
}

function validateDraft(draft: PromptWriteRequest): FieldErrors {
  const errors: FieldErrors = {};
  const name = draft.name.trim();
  const desc = draft.desc.trim();
  const value = draft.value.trim();
  if (!name) errors.name = '请输入提示词名称';
  else if ([...name].length > 80 || /[\r\n\t]/.test(name)) {
    errors.name = '名称最多 80 个字符，不能包含换行或制表符';
  }
  if (!desc) errors.desc = '请输入提示词描述';
  else if ([...desc].length > 500) errors.desc = '提示词描述最多 500 个字符';
  if (!value) errors.value = '请输入提示词内容';
  else if (new TextEncoder().encode(value).length > 16 * 1024) errors.value = '提示词内容最多 16KB';
  if (draft.tags.length > 2) errors.tags = '最多选择 2 个标签';
  return errors;
}

function PromptForm({
  draft,
  errors,
  onChange,
}: {
  draft: PromptWriteRequest;
  errors: FieldErrors;
  onChange: (draft: PromptWriteRequest) => void;
}) {
  const toggleTag = (tag: PromptTag) => {
    if (draft.tags.includes(tag)) {
      onChange({ ...draft, tags: draft.tags.filter((value) => value !== tag) });
    } else if (draft.tags.length < 2) {
      onChange({ ...draft, tags: [...draft.tags, tag] });
    }
  };
  const fieldClass = (field: keyof PromptWriteRequest) => cn(
    'w-full rounded-lg border bg-surface px-3 py-2 text-[13px] text-text-900 outline-none focus:border-accent focus:ring-2 focus:ring-accent/10',
    errors[field] ? 'border-danger' : 'border-border',
  );
  return (
    <div className="mx-auto grid w-full max-w-[820px] grid-cols-1 gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(220px,0.7fr)]">
      <label className="grid gap-1.5 text-xs font-semibold text-text-600 lg:col-span-2">
        名称
        <input
          autoFocus
          maxLength={80}
          value={draft.name}
          placeholder="例如：高管摘要 / Executive Summary"
          onChange={(event) => onChange({ ...draft, name: event.target.value })}
          className={fieldClass('name')}
          aria-invalid={Boolean(errors.name)}
        />
        {errors.name && <span className="font-medium text-danger">{errors.name}</span>}
      </label>
      <label className="grid gap-1.5 text-xs font-semibold text-text-600 lg:col-span-2">
        描述
        <textarea
          maxLength={500}
          rows={3}
          value={draft.desc}
          onChange={(event) => onChange({ ...draft, desc: event.target.value })}
          className={cn(fieldClass('desc'), 'resize-none leading-5')}
          aria-invalid={Boolean(errors.desc)}
        />
        {errors.desc && <span className="font-medium text-danger">{errors.desc}</span>}
      </label>
      <fieldset className="grid gap-1.5 lg:col-span-2">
        <legend className="mb-1.5 text-xs font-semibold text-text-600">标签（最多 2 个）</legend>
        <div className="flex flex-wrap gap-1.5">
          {promptTagOrder.map((tag) => {
            const selected = draft.tags.includes(tag);
            return (
              <button
                key={tag}
                type="button"
                aria-pressed={selected}
                disabled={!selected && draft.tags.length >= 2}
                onClick={() => toggleTag(tag)}
                className={cn(
                  'h-7 rounded-md border px-2.5 text-xs font-semibold disabled:cursor-not-allowed disabled:opacity-40',
                  selected ? 'border-accent/30 bg-accent-soft text-accent' : 'border-border bg-surface text-text-600 hover:bg-panel-muted',
                )}
              >
                {promptTagLabels[tag]}
              </button>
            );
          })}
        </div>
        {errors.tags && <span className="text-xs font-medium text-danger">{errors.tags}</span>}
      </fieldset>
      <label className="grid gap-1.5 text-xs font-semibold text-text-600 lg:col-span-2">
        提示词 value
        <textarea
          value={draft.value}
          onChange={(event) => onChange({ ...draft, value: event.target.value })}
          className={cn(fieldClass('value'), 'min-h-[220px] resize-y leading-6')}
          aria-invalid={Boolean(errors.value)}
        />
        {errors.value && <span className="font-medium text-danger">{errors.value}</span>}
      </label>
    </div>
  );
}

function PromptEditDialog({
  open,
  draft,
  errors,
  saving,
  onOpenChange,
  onChange,
  onSave,
}: {
  open: boolean;
  draft: PromptWriteRequest;
  errors: FieldErrors;
  saving: boolean;
  onOpenChange: (open: boolean) => void;
  onChange: (draft: PromptWriteRequest) => void;
  onSave: () => Promise<void>;
}) {
  return (
    <Dialog open={open} onOpenChange={(next) => !saving && onOpenChange(next)}>
      <DialogContent className="gap-0 p-0 sm:max-w-[720px]">
        <DialogHeader className="border-b border-border px-5 py-4">
          <DialogTitle>编辑提示词</DialogTitle>
        </DialogHeader>
        <form
          className="max-h-[75vh] overflow-y-auto px-5 py-5"
          onSubmit={(event) => {
            event.preventDefault();
            void onSave();
          }}
        >
          <PromptForm draft={draft} errors={errors} onChange={onChange} />
          <DialogFooter className="mt-5 border-t border-border pt-4">
            <Button type="button" variant="secondary" disabled={saving} onClick={() => onOpenChange(false)}>
              取消
            </Button>
            <Button type="submit" variant="primary" disabled={saving}>
              {saving ? '保存中' : '保存'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

export function PromptRepositoryPage() {
  const prompts = usePromptStore((state) => state.prompts);
  const loading = usePromptStore((state) => state.loading);
  const loaded = usePromptStore((state) => state.loaded);
  const error = usePromptStore((state) => state.error);
  const load = usePromptStore((state) => state.load);
  const createPrompt = usePromptStore((state) => state.create);
  const updatePrompt = usePromptStore((state) => state.update);
  const setPromptDisabled = usePromptStore((state) => state.setDisabled);
  const deletePrompt = usePromptStore((state) => state.delete);
  const [selectedId, setSelectedId] = useState('');
  const [query, setQuery] = useState('');
  const [filter, setFilter] = useState<PromptTag | 'all'>('all');
  const [mode, setMode] = useState<'view' | 'create'>('view');
  const [editOpen, setEditOpen] = useState(false);
  const [draft, setDraft] = useState<PromptWriteRequest>(emptyDraft);
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
  const [saving, setSaving] = useState(false);
  const [statusPending, setStatusPending] = useState(false);
  const [previousSelectedId, setPreviousSelectedId] = useState('');

  useEffect(() => {
    void load().catch(() => undefined);
  }, [load]);

  useEffect(() => {
    if (!selectedId && prompts[0]) setSelectedId(prompts[0].id);
  }, [prompts, selectedId]);

  const visible = useMemo(() => {
    const normalized = query.toLocaleLowerCase();
    return prompts.filter((prompt) => {
      if (filter !== 'all' && !prompt.tags.includes(filter)) return false;
      const searchable = [
        prompt.name, prompt.desc, prompt.value,
        ...prompt.tags.flatMap((tag) => [tag, promptTagLabels[tag]]),
      ].join(' ').toLocaleLowerCase();
      return searchable.includes(normalized);
    });
  }, [filter, prompts, query]);
  const visibleSelected = visible.find((prompt) => prompt.id === selectedId) ?? visible[0];
  const selected = mode === 'view'
    ? visibleSelected
    : prompts.find((prompt) => prompt.id === selectedId) ?? prompts[0];

  useEffect(() => {
    if (mode === 'view' && visibleSelected && visibleSelected.id !== selectedId) {
      setSelectedId(visibleSelected.id);
    }
  }, [mode, selectedId, visibleSelected]);

  const baseDraft = mode === 'create' ? emptyDraft : selected ? promptDraft(selected) : emptyDraft;
  const dirty = (mode === 'create' || editOpen) && !sameDraft(draft, baseDraft);
  const confirmDiscard = useCallback(() => !dirty || window.confirm('当前修改尚未保存，确定放弃吗？'), [dirty]);

  useEffect(() => {
    const beforeUnload = (event: BeforeUnloadEvent) => {
      if (!dirty) return;
      event.preventDefault();
    };
    window.addEventListener('beforeunload', beforeUnload);
    return () => window.removeEventListener('beforeunload', beforeUnload);
  }, [dirty]);

  const selectPrompt = (id: string) => {
    if (!confirmDiscard()) return;
    setMode('view');
    setEditOpen(false);
    setFieldErrors({});
    setSelectedId(id);
  };

  const beginCreate = () => {
    if (!confirmDiscard()) return;
    setPreviousSelectedId(selectedId);
    setMode('create');
    setDraft(emptyDraft);
    setFieldErrors({});
  };

  const beginEdit = () => {
    if (!selected) return;
    setDraft(promptDraft(selected));
    setFieldErrors({});
    setEditOpen(true);
  };

  const cancelCreate = () => {
    if (!confirmDiscard()) return;
    setMode('view');
    setSelectedId(previousSelectedId);
    setFieldErrors({});
  };

  const changeEditOpen = (open: boolean) => {
    if (!open && !confirmDiscard()) return;
    setEditOpen(open);
    if (!open) setFieldErrors({});
  };

  const save = async () => {
    if (saving) return;
    const next = {
      name: draft.name.trim(),
      desc: draft.desc.trim(),
      value: draft.value.trim(),
      tags: draft.tags,
    };
    const validation = validateDraft(next);
    setFieldErrors(validation);
    if (Object.keys(validation).length > 0) return;
    setSaving(true);
    try {
      const saved = mode === 'create'
        ? await createPrompt(next)
        : await updatePrompt(selectedId, next);
      setSelectedId(saved.id);
      setQuery('');
      setFilter('all');
      setMode('view');
      setEditOpen(false);
      setFieldErrors({});
    } catch (cause) {
      if (cause instanceof APIError && cause.code === 'PROMPT_NAME_CONFLICT') {
        const field = (cause.details as { field?: unknown } | undefined)?.field;
        if (field === 'name') {
          setFieldErrors((current) => ({ ...current, name: '该名称已存在' }));
          return;
        }
      }
      showGlobalError(cause instanceof Error ? cause.message : '提示词保存失败');
    } finally {
      setSaving(false);
    }
  };

  const remove = async (prompt: Prompt) => {
    const index = prompts.findIndex((value) => value.id === prompt.id);
    try {
      await deletePrompt(prompt.id);
      const remaining = usePromptStore.getState().prompts;
      setSelectedId(remaining[Math.min(index, remaining.length - 1)]?.id ?? '');
      setMode('view');
    } catch (cause) {
      showGlobalError(cause instanceof Error ? cause.message : '提示词删除失败');
      throw cause;
    }
  };

  const toggleDisabled = async (prompt: Prompt) => {
    if (statusPending) return;
    setStatusPending(true);
    try {
      await setPromptDisabled(prompt.id, !prompt.disabled);
    } catch (cause) {
      showGlobalError(cause instanceof Error ? cause.message : '提示词状态更新失败');
    } finally {
      setStatusPending(false);
    }
  };

  const createActions = (
    <div className="flex items-center gap-1.5">
      <Button type="button" variant="secondary" onClick={cancelCreate} disabled={saving} className="h-7 px-2.5 text-xs">
        <X className="h-3.5 w-3.5" />
        取消
      </Button>
      <Button type="button" variant="primary" onClick={() => void save()} disabled={saving} className="h-7 px-2.5 text-xs">
        <Check className="h-3.5 w-3.5" />
        {saving ? '保存中' : '保存'}
      </Button>
    </div>
  );

  return (
    <RepositoryShell section="prompt" onRefresh={() => {
      if (confirmDiscard()) {
        setMode('view');
        setEditOpen(false);
        void load(true).catch(() => undefined);
      }
    }}>
      <div className="flex h-full min-h-0 flex-col">
        <RepositoryPageHeader title="提示词" query={query} onQueryChange={setQuery} searchLabel="搜索提示词" />
        {error ? <RepositoryState text={error} error /> : loading || !loaded ? <RepositoryLoading aside /> : (
          <RepositoryWorkspace>
            <RepositoryCatalog
              label="提示词列表"
              controls={(['all', ...promptTagOrder] as const).map((tag) => (
                <RepositoryFilterButton key={tag} active={filter === tag} onClick={() => setFilter(tag)}>
                  {tag === 'all' ? '全部' : promptTagLabels[tag]}
                </RepositoryFilterButton>
              ))}
              footer={prompts.length > 0 ? (
                <Button type="button" variant="secondary" onClick={beginCreate} className="h-8 w-full border-dashed text-xs">
                  <Plus className="h-3.5 w-3.5" />
                  新建提示词
                </Button>
              ) : undefined}
            >
              {prompts.length === 0 ? (
                <div className="grid min-h-52 place-items-center">
                  <Button type="button" variant="secondary" onClick={beginCreate} className="border-dashed text-xs">
                    <Plus className="h-3.5 w-3.5" />
                    新建提示词
                  </Button>
                </div>
              ) : visible.length === 0 ? (
                <RepositoryState text="没有匹配的提示词" className="min-h-40" />
              ) : visible.map((prompt) => (
                <RepositoryDirectoryItem
                  key={prompt.id}
                  active={visibleSelected?.id === prompt.id}
                  disabled={prompt.disabled}
                  name={prompt.name}
                  description={prompt.desc}
                  visual={prompt.disabled
                    ? <Pause className="h-4 w-4" strokeWidth={1.75} />
                    : <NotebookText className="h-[17px] w-[17px]" strokeWidth={1.75} />}
                  visualBare={!prompt.disabled}
                  visualClassName={prompt.disabled
                    ? 'h-9 w-9 rounded-full border-0 bg-panel-muted text-text-400'
                    : 'text-accent'}
                  onClick={() => selectPrompt(prompt.id)}
                />
              ))}
            </RepositoryCatalog>
            {mode === 'create' ? (
              <section className="flex min-h-[420px] min-w-0 flex-col bg-surface" aria-label="新建提示词">
                <header className="flex min-h-[128px] shrink-0 items-start justify-between gap-6 border-b border-border px-5 pb-3.5 pt-5">
                  <h2 className="text-base font-bold leading-6 text-text-900">新建提示词</h2>
                  {createActions}
                </header>
                <div className="min-h-0 flex-1 overflow-auto bg-canvas/70 px-5 py-7 md:px-8 md:py-9">
                  <PromptForm draft={draft} errors={fieldErrors} onChange={setDraft} />
                </div>
              </section>
            ) : selected && (
              <RepositoryDetail
                label="提示词详情"
                title={selected.name}
                description={selected.desc}
                properties={(
                  <div className="flex flex-wrap gap-2" aria-label="标签">
                    {selected.tags.map((tag) => (
                      <span key={tag} className="rounded-md border border-border bg-panel px-1.5 py-0.5 text-[11px] font-semibold text-text-600">
                        {promptTagLabels[tag]}
                      </span>
                    ))}
                  </div>
                )}
                actions={(
                  <div className="flex items-center gap-2.5">
                    <span className={cn('text-xs font-semibold', selected.disabled ? 'text-text-600' : 'text-success')}>
                      {selected.disabled ? '已关闭' : '已启用'}
                    </span>
                    <button
                      type="button"
                      role="switch"
                      aria-checked={!selected.disabled}
                      aria-label="切换提示词状态"
                      disabled={statusPending}
                      onClick={() => void toggleDisabled(selected)}
                      className={cn(
                        'h-5 w-9 rounded-full p-0.5 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 disabled:opacity-50',
                        selected.disabled ? 'bg-border-strong' : 'bg-success',
                      )}
                    >
                      <span className={cn('block h-4 w-4 rounded-full bg-white shadow-[0_1px_3px_rgba(23,32,43,0.25)] transition-transform', !selected.disabled && 'translate-x-4')} />
                    </button>
                  </div>
                )}
                contentClassName="flex items-center justify-center p-5 md:p-8"
                deleteNoun="提示词"
                onEdit={beginEdit}
                onDelete={() => remove(selected)}
              >
                <article className="relative w-fit max-w-full rounded-xl border border-border bg-surface px-8 pb-8 pt-10 shadow-[0_18px_42px_rgba(51,65,85,0.11)] md:px-11 md:pb-10 md:pt-12">
                  <span aria-hidden="true" className="absolute left-6 top-5 h-[3px] w-7 rounded-full bg-accent md:left-7 md:top-6" />
                  <p className="m-0 whitespace-pre-wrap break-words text-lg font-semibold leading-8 tracking-[0.01em] text-[#243142] md:text-xl md:leading-9">
                    {selected.value}
                  </p>
                </article>
              </RepositoryDetail>
            )}
            {mode === 'view' && !selected && prompts.length > 0 && (
              <RepositoryState text="请选择一个提示词" className="min-h-[420px] bg-canvas/70" />
            )}
          </RepositoryWorkspace>
        )}
      </div>
      <PromptEditDialog
        open={editOpen}
        draft={draft}
        errors={fieldErrors}
        saving={saving}
        onOpenChange={changeEditOpen}
        onChange={setDraft}
        onSave={save}
      />
    </RepositoryShell>
  );
}
