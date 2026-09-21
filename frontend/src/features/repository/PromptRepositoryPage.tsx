import { NotebookText, Pause } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import type { Prompt, PromptTag } from '../../api/types';
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
  RepositoryTagList,
  RepositoryWorkspace,
} from './RepositoryPrimitives';
import { RepositoryEditDialog } from './RepositoryEditDialog';
import { RepositoryShell } from './RepositoryShell';

export function PromptRepositoryPage() {
  const prompts = usePromptStore((state) => state.prompts);
  const loading = usePromptStore((state) => state.loading);
  const loaded = usePromptStore((state) => state.loaded);
  const error = usePromptStore((state) => state.error);
  const load = usePromptStore((state) => state.load);
  const updatePrompt = usePromptStore((state) => state.update);
  const setPromptDisabled = usePromptStore((state) => state.setDisabled);
  const deletePrompt = usePromptStore((state) => state.delete);
  const [selectedId, setSelectedId] = useState('');
  const [query, setQuery] = useState('');
  const [filter, setFilter] = useState<PromptTag | 'all'>('all');
  const [editOpen, setEditOpen] = useState(false);
  const [statusPending, setStatusPending] = useState(false);

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
  const selected = visible.find((prompt) => prompt.id === selectedId) ?? visible[0];

  useEffect(() => {
    if (selected && selected.id !== selectedId) {
      setSelectedId(selected.id);
    }
  }, [selected, selectedId]);

  const updatePromptMetadata = async (value: { name: string; description: string; tags: PromptTag[] }) => {
    if (!selected) return;
    try {
      await updatePrompt(selected.id, {
        name: value.name,
        desc: value.description,
        value: selected.value,
        tags: value.tags,
      });
    } catch (cause) {
      showGlobalError(cause instanceof Error ? cause.message : '提示词更新失败');
      throw cause;
    }
  };

  const remove = async (prompt: Prompt) => {
    const index = prompts.findIndex((value) => value.id === prompt.id);
    try {
      await deletePrompt(prompt.id);
      const remaining = usePromptStore.getState().prompts;
      setSelectedId(remaining[Math.min(index, remaining.length - 1)]?.id ?? '');
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

  return (
    <RepositoryShell section="prompt" onRefresh={() => {
      setEditOpen(false);
      void load(true).catch(() => undefined);
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
            >
              {visible.length === 0 ? (
                <RepositoryState
                  text={prompts.length === 0 ? '暂无提示词' : '没有匹配的提示词'}
                  className="min-h-40"
                />
              ) : visible.map((prompt) => (
                <RepositoryDirectoryItem
                  key={prompt.id}
                  active={selected?.id === prompt.id}
                  disabled={prompt.disabled}
                  name={prompt.name}
                  description={prompt.desc}
                  visual={prompt.disabled
                    ? <Pause className="h-4 w-4" strokeWidth={1.75} />
                    : <NotebookText className="h-4 w-4" strokeWidth={1.75} />}
                  visualClassName={prompt.disabled
                    ? 'h-9 w-9 rounded-full border-0 bg-panel-muted text-text-400'
                    : 'h-9 w-9 rounded-full border-0 bg-accent-soft text-accent'}
                  onClick={() => setSelectedId(prompt.id)}
                />
              ))}
            </RepositoryCatalog>
            {selected && (
              <RepositoryDetail
                label="提示词详情"
                title={selected.name}
                description={selected.desc}
                properties={<RepositoryTagList tags={selected.tags.map((tag) => promptTagLabels[tag])} />}
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
                onEdit={() => setEditOpen(true)}
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
            {!selected && prompts.length > 0 && (
              <RepositoryState text="请选择一个提示词" className="min-h-[420px] bg-canvas/70" />
            )}
          </RepositoryWorkspace>
        )}
      </div>
      {selected && (
        <RepositoryEditDialog
          open={editOpen}
          title="编辑提示词"
          value={{ name: selected.name, description: selected.desc, tags: selected.tags }}
          tagOptions={promptTagOrder.map((tag) => ({ value: tag, label: promptTagLabels[tag] }))}
          onOpenChange={setEditOpen}
          onSave={updatePromptMetadata}
        />
      )}
    </RepositoryShell>
  );
}
