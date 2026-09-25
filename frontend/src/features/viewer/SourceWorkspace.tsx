import React from 'react';
import { Copy, Ellipsis, Save } from 'lucide-react';
import { SOURCE_META, isProjectSource, sourceOrder, type SourceKind } from '../../api/slideSources';
import { SourceEditor } from '../../components/SourceEditor';
import { Button, InlineNotice } from '../../components/ui/primitives';
import { enqueueSourceDraft, listSourceDrafts, sourceDraftId, type StoredSourceDraft } from '../../lib/sourceDraftStorage';
import { projectSourceDraftCount, showSourceFile, sourceDirty, sourceKey, useSourceEditorStore } from '../../stores/sourceEditorStore';
import { useProjectStore } from '../../stores/projectStore';
import { orderedSlides } from '../deck/selectors';

export function SourceWorkspace({ projectId, slideId = '', kind, blocked }: { projectId: string; slideId?: string; kind: SourceKind; blocked: boolean }) {
  const key = sourceKey(projectId, slideId, kind);
  const file = useSourceEditorStore((state) => state.files[key]);
  const drafts = useSourceEditorStore((state) => state.drafts);
  const allFiles = useSourceEditorStore((state) => state.files);
  const load = useSourceEditorStore((state) => state.load);
  const save = useSourceEditorStore((state) => state.save);
  const saveAll = useSourceEditorStore((state) => state.saveAll);
  const setDraft = useSourceEditorStore((state) => state.setDraft);
  const snapshot = useProjectStore((state) => state.contentByProjectId[projectId]);
  const slides = React.useMemo(() => orderedSlides(snapshot), [snapshot]);
  const [showDrafts, setShowDrafts] = React.useState(false);
  const [backups, setBackups] = React.useState<StoredSourceDraft[]>([]);
  const [showBackups, setShowBackups] = React.useState(false);
  const [selectedBackup, setSelectedBackup] = React.useState<StoredSourceDraft | null>(null);

  React.useEffect(() => { void load(projectId, slideId, kind); }, [projectId, slideId, kind, load]);
  React.useEffect(() => { return () => { void useSourceEditorStore.getState().flush(key); }; }, [key]);
  React.useEffect(() => {
    void listSourceDrafts().then((rows) => setBackups(rows.filter((item) => item.projectId === projectId && item.backup).sort((a, b) => b.updatedAt - a.updatedAt))).catch(() => {});
  }, [projectId, file?.sceneRevision]);
  const activeDrafts = [...new Map<string, { id: string; slideId: string; kind: SourceKind }>([
    ...Object.values(drafts).filter((item) => item.projectId === projectId && item.draftText !== item.baseText).map((item) => [item.id, { id: item.id, slideId: item.slideId, kind: item.kind }] as const),
    ...Object.values(allFiles).filter((item) => item.projectId === projectId && sourceDirty(item)).map((item) => {
      const id = sourceDraftId(item.projectId, item.slideId, item.kind);
      return [id, { id, slideId: item.slideId, kind: item.kind }] as const;
    }),
  ]).values()].sort((a, b) => sourceOrder(a, slides.map((slide) => slide.id)) - sourceOrder(b, slides.map((slide) => slide.id)));
  const titleFor = (draft: { slideId: string; kind: SourceKind }) => isProjectSource(draft.kind) ? SOURCE_META[draft.kind].label : slides.find((slide) => slide.id === draft.slideId)?.title || draft.slideId;
  const dirtyCount = projectSourceDraftCount(projectId);
  const readonly = blocked || !file?.writable || file.phase !== 'ready';
  const status = blocked ? 'Agent 任务进行中，暂不可编辑' : file?.phase === 'missing' ? '源文件尚未生成' : !file || file.phase === 'loading' || file.phase === 'reloading' ? '正在加载最新内容…' : file.phase === 'formatting' ? '正在格式化…' : file.phase === 'saving' ? '正在保存…' : file.draftPersistence === 'error' ? '草稿暂存失败' : file.error ? '无法保存' : sourceDirty(file) ? '未保存' : '已保存';
  const filename = SOURCE_META[kind].filename;
  const relativePath = isProjectSource(kind) ? filename : `slides/${slideId}/${filename}`;

  return <div className="flex min-h-0 min-w-0 flex-1 flex-col bg-surface">
    <div className="flex min-h-11 flex-wrap items-center gap-2 border-b border-border bg-panel px-3 py-1.5 text-xs">
      <span className="min-w-0 truncate font-mono font-semibold text-text-900" title={relativePath}>{filename}</span>
      <span className="text-text-400">{SOURCE_META[kind].language}</span>
      <span className="ml-auto text-text-600" role="status">{status}</span>
      {dirtyCount > 0 && <button type="button" className="text-accent hover:underline" onClick={() => setShowDrafts((value) => !value)}>未保存 {dirtyCount} 个</button>}
      <Button type="button" variant="primary" className="h-7 px-3 text-xs" disabled={readonly} onClick={() => void save(key)}><Save className="mr-1 h-3.5 w-3.5" strokeWidth={1.75} />保存</Button>
      {backups.length > 0 && <button type="button" aria-label="历史草稿" title="历史草稿" onClick={() => setShowBackups((value) => !value)}><Ellipsis className="h-4 w-4" strokeWidth={1.75} /></button>}
    </div>
    {showDrafts && <div className="border-b border-border bg-panel px-3 py-2 text-xs">
      {activeDrafts.map((draft) => <button key={draft.id} type="button" className="mr-3 rounded px-1.5 py-1 text-text-700 hover:bg-accent-soft" onClick={() => {
        showSourceFile(draft); setShowDrafts(false);
      }}>{titleFor(draft)} · {SOURCE_META[draft.kind].filename}</button>)}
      <button type="button" className="text-accent hover:underline" onClick={() => void saveAll(projectId)}>保存全部</button>
    </div>}
    {showBackups && <div className="border-b border-border bg-panel px-3 py-2 text-xs">
      <div className="mb-2 flex justify-between"><strong>历史草稿</strong><button type="button" onClick={() => setShowBackups(false)}>关闭</button></div>
      {backups.length === 0 && <span className="text-text-400">没有历史草稿</span>}
      {backups.map((draft) => <div key={draft.id} className="flex items-center gap-2 py-1">
        <button type="button" className="truncate text-left hover:underline" onClick={() => setSelectedBackup(draft)}>{titleFor(draft)} · {SOURCE_META[draft.kind].filename} · {new Date(draft.updatedAt).toLocaleString()}</button>
        <button type="button" aria-label="复制历史草稿" title="复制历史草稿" onClick={() => void navigator.clipboard.writeText(draft.draftText)}><Copy className="h-3.5 w-3.5" /></button>
        <button type="button" className="text-danger" onClick={() => void enqueueSourceDraft(draft.id, null).then(() => setBackups((rows) => rows.filter((row) => row.id !== draft.id)))}>删除</button>
      </div>)}
      {selectedBackup && <pre className="scrollbar-none mt-2 max-h-48 overflow-auto whitespace-pre font-mono text-xs" aria-label="只读历史草稿">{selectedBackup.draftText}</pre>}
    </div>}
    {file?.phase === 'missing' ? <div className="flex flex-1 items-center justify-center text-sm text-text-400">{SOURCE_META[kind].label}源文件尚未生成</div>
      : !file || (file.phase === 'error' && !file.baseSourceHash) ? <div className="flex flex-1 flex-col items-center justify-center gap-3 text-sm text-text-600">{file?.error || '正在加载源文件…'}<Button variant="secondary" onClick={() => void load(projectId, slideId, kind, true)}>重试</Button></div>
      : <>
        {file.error && <InlineNotice tone="danger" className="m-2 text-xs">{file.error} <button type="button" className="ml-2 underline" onClick={() => void load(projectId, slideId, kind, true)}>重试</button></InlineNotice>}
        <div className="min-h-0 min-w-0 flex-1">
          <SourceEditor key={`${key}:${file.resetVersion}`} resourceKey={key} kind={kind} text={file.draftText} resetVersion={file.resetVersion}
            readOnly={readonly} selectionAnchor={file.selectionAnchor} scrollTop={file.scrollTop} diagnostics={file.diagnostics}
            onChange={(text, anchor, scroll) => setDraft(key, text, anchor, scroll, file.resetVersion)} onSave={() => { if (!readonly) void save(key); }} onFlush={() => void useSourceEditorStore.getState().flush(key)} />
        </div>
      </>}
  </div>;
}
