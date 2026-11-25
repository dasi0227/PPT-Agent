import { useCallback, useEffect, useRef, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { BookOpenText, Component, NotebookText, Palette, Plus, Tags } from 'lucide-react';
import { assetsApi, type AssetDetail, type AssetDraft, type AssetInfo } from '../../api/assets';
import type { ResourceScope } from '../../api/types';
import { IconButton } from '../../components/ui/primitives';
import { Switch } from '../../components/ui/switch';
import { showGlobalError } from '../../stores/toastStore';
import { useTagStore } from '../../stores/tagStore';
import { cn } from '../../lib/utils';
import { RepositoryShell } from './RepositoryShell';
import { RepositoryCatalog, RepositoryDetail, RepositoryDirectoryItem, RepositoryFilterButton, RepositoryLoading, RepositoryPageHeader, RepositoryState, RepositoryTagList, RepositoryWorkspace } from './RepositoryPrimitives';
import { RepositoryEditDialog } from './RepositoryEditDialog';
import { RepositoryTagsDialog } from './RepositoryTagsDialog';
import { AssetPreview } from './AssetPreview';

const sections = {
  theme: ['主题', Palette], component: ['组件', Component], skill: ['技能', BookOpenText], snippet: ['短语', NotebookText],
} as const;
const blank: AssetDraft = { name: '', description: '', tag_ids: [], content: '' };
type Editor = { id?: string; value: AssetDraft };
const detailClasses = {
  theme: 'flex flex-col p-5 md:p-7 xl:p-9', component: 'flex items-center justify-center overflow-hidden p-5 md:p-7',
  skill: 'px-5 py-7 md:px-8 md:py-9', snippet: 'flex items-center justify-center p-5 md:p-8',
};
export function AssetRepositoryPage({ type, active = true }: { type: ResourceScope; active?: boolean }) {
  const [title, icon] = sections[type];
  const [params] = useSearchParams();
  const requested = params.get('id') ?? '';
  const [selectedId, select] = useState(requested);
  const [query, search] = useState('');
  const [filter, setFilter] = useState('');
  const [items, setItems] = useState<AssetInfo[]>([]);
  const [preview, setPreview] = useState<AssetDetail>();
  const [previewError, setPreviewError] = useState('');
  const [refreshVersion, setRefreshVersion] = useState(0);
  const [editor, setEditor] = useState<Editor>();
  const [manageTags, setManageTags] = useState(false);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const listVersion = useRef(0);
  const editVersion = useRef(0);
  const tags = useTagStore(state => state.tags[type]) ?? [];
  const loadTags = useTagStore(state => state.load);
  const loadList = useCallback(async () => {
    const version = ++listVersion.current;
    try {
      const { items } = await assetsApi.list(type, query);
      if (version === listVersion.current) { setItems(items); setError(''); }
    } catch (err) { if (version === listVersion.current) setError(err instanceof Error ? err.message : '仓库加载失败'); }
    finally { if (version === listVersion.current) setLoading(false); }
  }, [type, query]);
  useEffect(() => {
    if (active) void loadList();
    return () => { ++listVersion.current; };
  }, [active, loadList]);
  useEffect(() => {
    if (active) void loadTags(type).catch(err => showGlobalError(err.message));
    else { setEditor(undefined); setManageTags(false); }
    return () => { ++editVersion.current; };
  }, [active, loadTags, type]);
  useEffect(() => { if (requested) { select(requested); search(''); setFilter(''); } }, [requested]);
  const visible = filter && tags.some(tag => tag.id === filter) ? items.filter(item => item.tag_ids.includes(filter)) : items;
  const selected = visible.find(item => item.id === selectedId) ?? visible[0];
  const id = selected?.id;
  const updatedAt = selected?.updated_at;
  const currentPreview = preview && preview.id === id && preview.updated_at === updatedAt ? preview : undefined;
  useEffect(() => {
    if (!active || !id || updatedAt === undefined || currentPreview) return;
    let current = true;
    setPreviewError('');
    void assetsApi.detail(id).then(detail => {
      if (current) {
        setPreview(detail);
        if (detail.updated_at !== updatedAt) {
          setItems(rows => rows.map(row => row.id === id ? detail : row));
        }
      }
    }).catch(err => { if (current) setPreviewError(err.message); });
    return () => { current = false; };
  }, [active, id, updatedAt, currentPreview, refreshVersion]);
  const refresh = async () => {
    setPreview(undefined); setPreviewError(''); setRefreshVersion(version => version + 1);
    await Promise.all([loadTags(type), loadList()]);
  };
  async function edit(item: AssetInfo) {
    const version = ++editVersion.current;
    try {
      const { id, name, description, tag_ids, content } = await assetsApi.detail(item.id);
      if (editVersion.current === version) setEditor({ id, value: { name, description, tag_ids, content } });
    } catch (err) { showGlobalError(err instanceof Error ? err.message : '正文加载失败'); }
  }
  async function refreshSavedAsset(id: string) {
    try {
      const detail = await assetsApi.detail(id);
      ++listVersion.current;
      setItems(rows => [detail, ...rows.filter(row => row.id !== id)]);
      setPreview(detail);
    } catch (cause) {
      showGlobalError('资产已保存，读取详情失败。' + (cause instanceof Error ? cause.message : ''));
      await loadList();
    }
  }
  async function save(draft: AssetDraft) {
    if (!editor) return;
    const { id, value } = editor;
    const changes = {
      ...(draft.name !== value.name && { name: draft.name }),
      ...(draft.description !== value.description && { description: draft.description }),
      ...(draft.tag_ids !== value.tag_ids && { tag_ids: draft.tag_ids }),
      ...(draft.content !== value.content && { content: draft.content }),
    };
    if (id && !Object.keys(changes).length) return;
    let savedId = id;
    if (savedId) await assetsApi.update(savedId, type, changes);
    else savedId = (await assetsApi.create(type, draft)).id;
    await refreshSavedAsset(savedId);
    select(savedId);
    search(''); setFilter('');
  }
  async function toggle(item: AssetInfo) {
    if (busy) return;
    setBusy(true);
    try {
      await assetsApi.update(item.id, item.type, { enabled: !item.enabled });
      await refreshSavedAsset(item.id);
    } catch (err) { showGlobalError(err instanceof Error ? err.message : '状态更新失败'); }
    finally { setBusy(false); }
  }
  async function remove(item: AssetInfo) {
    try {
      await assetsApi.delete(item.id);
      setItems(current => current.filter(row => row.id !== item.id));
    } catch (err) { showGlobalError(err instanceof Error ? err.message : '删除失败'); throw err; }
  }
  return <RepositoryShell section={type} onRefresh={() => void refresh().catch(err => showGlobalError(err.message))}>
    <div className="flex h-full min-h-0 flex-col">
      <RepositoryPageHeader title={title} query={query} onQueryChange={search} searchLabel={`搜索${title}`} actions={<>
        <IconButton label="管理标签" onClick={() => setManageTags(true)}><Tags size={16} /></IconButton>
        <IconButton label={`创建${title}`} onClick={() => setEditor({ value: blank })}><Plus size={16} /></IconButton>
      </>} />
      {loading ? <RepositoryLoading aside /> : error ? <RepositoryState text={error} error /> : <RepositoryWorkspace>
        <RepositoryCatalog label={title + '列表'} controls={<>
          <RepositoryFilterButton active={!filter} onClick={() => setFilter('')}>全部</RepositoryFilterButton>
          {tags.map(tag => <RepositoryFilterButton key={tag.id} active={filter === tag.id} onClick={() => setFilter(tag.id)}>{tag.name}</RepositoryFilterButton>)}
        </>}>
          {visible.length ? visible.map(item => <RepositoryDirectoryItem key={item.id} active={selected?.id === item.id} disabled={!item.enabled}
            name={item.name} description={item.description} descriptionLines={type === 'skill' ? 3 : 2} icon={icon} onClick={() => select(item.id)} />)
            : <RepositoryState text={type === 'snippet' && !query && !filter ? '暂无短语' : '没有匹配的' + title} className="min-h-40" />}
        </RepositoryCatalog>
        {selected ? <RepositoryDetail key={selected.id} active={active} label={type === 'skill' ? '技能说明' : title + '详情'} title={selected.name} description={selected.description}
          properties={<RepositoryTagList tags={selected.tag_ids.flatMap(id => tags.find(tag => tag.id === id)?.name ?? [])} />}
          actions={<div className="flex items-center gap-2.5">
            <span className={cn('text-xs font-semibold', selected.enabled ? 'text-success' : 'text-text-600')}>{selected.enabled ? '已启用' : '已关闭'}</span>
            <Switch checked={selected.enabled} aria-label={`切换${title}状态`} disabled={busy} onCheckedChange={() => void toggle(selected)} />
          </div>} contentClassName={detailClasses[type]} deleteNoun={title} onEdit={() => void edit(selected)} onDelete={() => remove(selected)}>
          {currentPreview ? <AssetPreview item={currentPreview} /> : previewError ? <RepositoryState text={previewError} error /> : <RepositoryLoading />}
        </RepositoryDetail> : <RepositoryState text={`请选择一个${title}`} className="min-h-[420px] bg-canvas/70" />}
      </RepositoryWorkspace>}
    </div>
    {active && editor && <RepositoryEditDialog title={(editor.id ? '编辑' : '创建') + title} value={editor.value} tags={tags} onClose={() => setEditor(undefined)} onSave={save} />}
    {active && manageTags && <RepositoryTagsDialog type={type} tags={tags} onClose={() => setManageTags(false)} onChange={async () => { setFilter(''); await refresh(); }} />}
  </RepositoryShell>;
}
