import { NotebookText } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import type { Snippet, SnippetTag } from '../../api/types';
import { cn } from '../../lib/utils';
import { useSnippetStore } from '../../stores/snippetStore';
import { showGlobalError } from '../../stores/toastStore';
import { snippetTagLabels, snippetTagOrder } from '../agent/promptMatching';
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

export function SnippetRepositoryPage() {
  const snippets = useSnippetStore((state) => state.snippets);
  const loading = useSnippetStore((state) => state.loading);
  const loaded = useSnippetStore((state) => state.loaded);
  const error = useSnippetStore((state) => state.error);
  const load = useSnippetStore((state) => state.load);
  const updateSnippet = useSnippetStore((state) => state.update);
  const setSnippetDisabled = useSnippetStore((state) => state.setDisabled);
  const deleteSnippet = useSnippetStore((state) => state.delete);
  const [selectedId, setSelectedId] = useState('');
  const [query, setQuery] = useState('');
  const [filter, setFilter] = useState<SnippetTag | 'all'>('all');
  const [editOpen, setEditOpen] = useState(false);
  const [statusPending, setStatusPending] = useState(false);

  useEffect(() => {
    void load().catch(() => undefined);
  }, [load]);

  useEffect(() => {
    if (!selectedId && snippets[0]) setSelectedId(snippets[0].id);
  }, [snippets, selectedId]);

  const visible = useMemo(() => {
    const normalized = query.toLocaleLowerCase();
    return snippets.filter((snippet) => {
      if (filter !== 'all' && !snippet.tags.includes(filter)) return false;
      const searchable = [
        snippet.name, snippet.description, snippet.content,
        ...snippet.tags.flatMap((tag) => [tag, snippetTagLabels[tag]]),
      ].join(' ').toLocaleLowerCase();
      return searchable.includes(normalized);
    });
  }, [filter, snippets, query]);
  const selected = visible.find((snippet) => snippet.id === selectedId) ?? visible[0];

  useEffect(() => {
    if (selected && selected.id !== selectedId) {
      setSelectedId(selected.id);
    }
  }, [selected, selectedId]);

  const updateSnippetMetadata = async (value: { name: string; description: string; tags: SnippetTag[] }) => {
    if (!selected) return;
    try {
      await updateSnippet(selected.id, {
        name: value.name,
        description: value.description,
        tags: value.tags,
      });
    } catch (cause) {
      showGlobalError(cause instanceof Error ? cause.message : '短语更新失败');
      throw cause;
    }
  };

  const remove = async (snippet: Snippet) => {
    const index = snippets.findIndex((value) => value.id === snippet.id);
    try {
      await deleteSnippet(snippet.id);
      const remaining = useSnippetStore.getState().snippets;
      setSelectedId(remaining[Math.min(index, remaining.length - 1)]?.id ?? '');
    } catch (cause) {
      showGlobalError(cause instanceof Error ? cause.message : '短语删除失败');
      throw cause;
    }
  };

  const toggleDisabled = async (snippet: Snippet) => {
    if (statusPending) return;
    setStatusPending(true);
    try {
      await setSnippetDisabled(snippet.id, !snippet.disabled);
    } catch (cause) {
      showGlobalError(cause instanceof Error ? cause.message : '短语状态更新失败');
    } finally {
      setStatusPending(false);
    }
  };

  return (
    <RepositoryShell section="snippet" onRefresh={() => {
      setEditOpen(false);
      void load(true).catch(() => undefined);
    }}>
      <div className="flex h-full min-h-0 flex-col">
        <RepositoryPageHeader title="短语" query={query} onQueryChange={setQuery} searchLabel="搜索短语" />
        {error ? <RepositoryState text={error} error /> : loading || !loaded ? <RepositoryLoading aside /> : (
          <RepositoryWorkspace>
            <RepositoryCatalog
              label="短语列表"
              controls={(['all', ...snippetTagOrder] as const).map((tag) => (
                <RepositoryFilterButton key={tag} active={filter === tag} onClick={() => setFilter(tag)}>
                  {tag === 'all' ? '全部' : snippetTagLabels[tag]}
                </RepositoryFilterButton>
              ))}
            >
              {visible.length === 0 ? (
                <RepositoryState
                  text={snippets.length === 0 ? '暂无短语' : '没有匹配的短语'}
                  className="min-h-40"
                />
              ) : visible.map((snippet) => (
                <RepositoryDirectoryItem
                  key={snippet.id}
                  active={selected?.id === snippet.id}
                  disabled={snippet.disabled}
                  name={snippet.name}
                  description={snippet.description}
                  icon={NotebookText}
                  onClick={() => setSelectedId(snippet.id)}
                />
              ))}
            </RepositoryCatalog>
            {selected && (
              <RepositoryDetail
                label="短语详情"
                title={selected.name}
                description={selected.description}
                openUrl={selected.open_url}
                properties={<RepositoryTagList tags={selected.tags.map((tag) => snippetTagLabels[tag])} />}
                actions={(
                  <div className="flex items-center gap-2.5">
                    <span className={cn('text-xs font-semibold', selected.disabled ? 'text-text-600' : 'text-success')}>
                      {selected.disabled ? '已关闭' : '已启用'}
                    </span>
                    <button
                      type="button"
                      role="switch"
                      aria-checked={!selected.disabled}
                      aria-label="切换短语状态"
                      disabled={statusPending}
                      onClick={() => void toggleDisabled(selected)}
                      className={cn(
                        'h-5 w-9 rounded-full p-0.5 transition-colors focus-visible:outline-none disabled:opacity-50',
                        selected.disabled ? 'bg-border-strong' : 'bg-success',
                      )}
                    >
                      <span className={cn('block h-4 w-4 rounded-full bg-white shadow-[0_1px_3px_rgba(23,32,43,0.25)] transition-transform', !selected.disabled && 'translate-x-4')} />
                    </button>
                  </div>
                )}
                contentClassName="flex items-center justify-center p-5 md:p-8"
                deleteNoun="短语"
                onEdit={() => setEditOpen(true)}
                onDelete={() => remove(selected)}
              >
                {selected.content_state !== 'ready' ? (
                  <RepositoryState text={selected.content_error ?? '资源文件不可用'} error />
                ) : (<>
                <article className="w-fit max-w-full rounded-xl border border-border bg-surface px-8 py-8 shadow-[0_18px_42px_rgba(51,65,85,0.11)] md:px-11 md:py-10">
                  <p className="m-0 whitespace-pre-wrap break-words text-lg font-semibold leading-8 tracking-[0.01em] text-[#243142] md:text-xl md:leading-9">
                    {selected.content}
                  </p>
                </article>
                </>)}
              </RepositoryDetail>
            )}
            {!selected && snippets.length > 0 && (
              <RepositoryState text="请选择一个短语" className="min-h-[420px] bg-canvas/70" />
            )}
          </RepositoryWorkspace>
        )}
      </div>
      {selected && (
        <RepositoryEditDialog
          open={editOpen}
          title="编辑短语"
          value={{ name: selected.name, description: selected.description, tags: selected.tags }}
          tagOptions={snippetTagOrder.map((tag) => ({ value: tag, label: snippetTagLabels[tag] }))}
          onOpenChange={setEditOpen}
          onSave={updateSnippetMetadata}
        />
      )}
    </RepositoryShell>
  );
}
