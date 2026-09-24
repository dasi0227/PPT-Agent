import { create } from 'zustand';
import { APIError, NetworkError, RequestTimeoutError } from '../api/client';
import { projectHistoryApi } from '../api/projectHistory';
import { slideSourcesApi, type SlideSourceDocument, type SourceKind } from '../api/slideSources';
import { backupProjectSourceDrafts, enqueueSourceDraft, flushSourceDrafts, listSourceDrafts, sourceDraftId, sourceSessionId, type StoredSourceDraft } from '../lib/sourceDraftStorage';
import { formatHTML, formatStrictJSON, SourceFormatError } from '../lib/sourceFormatting';
import { useDeckStore } from './deckStore';
import { useProjectStore } from './projectStore';
import { showGlobalSuccess, showGlobalWarning } from './toastStore';

export type SourcePhase = 'loading' | 'ready' | 'formatting' | 'saving' | 'reloading' | 'missing' | 'error';
export interface SourceFile {
  projectId: string;
  slideId: string;
  kind: SourceKind;
  phase: SourcePhase;
  sceneRevision: number;
  baseSourceHash: string;
  baseContentHash: string | null;
  baseText: string;
  draftText: string;
  editGeneration: number;
  resetVersion: number;
  writable: boolean;
  readonlyReason: string | null;
  draftPersistence: 'pending' | 'saved' | 'error';
  error?: string;
  diagnostics?: { from: number; to: number; message: string; severity: 'error' | 'warning' }[];
  selectionAnchor?: number;
  scrollTop?: number;
}

export const sourceKey = (projectId: string, slideId: string, kind: SourceKind) => `${projectId}:${slideId}:${kind}`;
export const sourceDirty = (file: SourceFile) => file.draftText !== file.baseText;
const persistenceTimers = new Map<string, number>();
const readTokens = new Map<string, number>();
let indexPromise: Promise<void> | null = null;

interface SourceEditorState {
  files: Record<string, SourceFile>;
  drafts: Record<string, StoredSourceDraft>;
  indexReady: boolean;
  indexError: string | null;
  ensureIndex: () => Promise<void>;
  load: (projectId: string, slideId: string, kind: SourceKind, force?: boolean) => Promise<void>;
  setDraft: (key: string, text: string, selectionAnchor?: number, scrollTop?: number, expectedResetVersion?: number) => void;
  flush: (key?: string) => Promise<void>;
  save: (key: string) => Promise<boolean>;
  saveAll: (projectId: string) => Promise<boolean>;
  refreshProject: (projectId: string) => Promise<void>;
}

function draftRecord(file: SourceFile): StoredSourceDraft {
  return {
    id: sourceDraftId(file.projectId, file.slideId, file.kind), sessionId: sourceSessionId(),
    projectId: file.projectId, slideId: file.slideId, kind: file.kind, sceneRevision: file.sceneRevision,
    baseSourceHash: file.baseSourceHash, baseText: file.baseText, draftText: file.draftText,
    selectionAnchor: file.selectionAnchor, scrollTop: file.scrollTop, updatedAt: Date.now(),
  };
}

function message(error: unknown): string { return error instanceof Error ? error.message : '源文件操作失败'; }

function serverDiagnostic(error: APIError, text: string, candidate?: string): SourceFile['diagnostics'] {
  const details = error.details as { diagnostics?: { message?: string; json_pointer?: string; line?: number; column?: number }[] } | undefined;
  const first = details?.diagnostics?.[0];
  if (!first) return undefined;
  let from: number | undefined;
  let to: number | undefined;
  if (first.json_pointer) {
    const segment = first.json_pointer.split('/').pop()?.replace(/~1/g, '/').replace(/~0/g, '~');
    if (segment) {
      const encoded = JSON.stringify(segment);
      const index = text.indexOf(encoded);
      if (index >= 0) { from = index; to = index + encoded.length; }
    }
  }
  if (from === undefined && candidate === text && first.line && first.column) {
    const lines = text.split('\n');
    if (first.line <= lines.length) {
      from = lines.slice(0, first.line - 1).reduce((sum, line) => sum + line.length + 1, 0) + first.column - 1;
      to = Math.min(text.length, from + 1);
    }
  }
  return from === undefined ? undefined : [{ from, to: to ?? from + 1, message: first.message || error.message, severity: 'error' }];
}

export const useSourceEditorStore = create<SourceEditorState>((set, get) => ({
  files: {}, drafts: {}, indexReady: false, indexError: null,
  ensureIndex: async () => {
    if (get().indexReady) return;
    if (!indexPromise) indexPromise = (async () => {
      try {
        const rows = await listSourceDrafts();
        set({ drafts: Object.fromEntries(rows.filter((row) => !row.backup).map((row) => [row.id, row])), indexReady: true, indexError: null });
      } catch (error) {
        set({ indexReady: false, indexError: message(error) });
        throw error;
      }
    })().finally(() => { indexPromise = null; });
    await indexPromise;
  },
  load: async (projectId, slideId, kind, force = false) => {
    const key = sourceKey(projectId, slideId, kind);
    await get().ensureIndex();
    const before = get().files[key];
    if (!force && before?.phase === 'ready') return;
    if (before?.phase === 'formatting' || before?.phase === 'saving') return;
    const token = (readTokens.get(key) ?? 0) + 1;
    readTokens.set(key, token);
    set((state) => ({ files: { ...state.files, [key]: before ? { ...before, phase: 'reloading' } : {
      projectId, slideId, kind, phase: 'loading', sceneRevision: 0, baseSourceHash: '', baseContentHash: null,
      baseText: '', draftText: '', editGeneration: 0, resetVersion: 0, writable: false,
      readonlyReason: null, draftPersistence: 'saved',
    } } }));
    try {
      const document = await slideSourcesApi.get(projectId, slideId, kind);
      if (readTokens.get(key) !== token) return;
      const current = get().files[key];
      if (current?.phase === 'saving' || current?.phase === 'formatting') return;
      const stored = get().drafts[sourceDraftId(projectId, slideId, kind)];
      const sameCurrent = current?.baseSourceHash === document.source_hash && current.sceneRevision === document.scene_revision;
      const sameStored = stored?.baseSourceHash === document.source_hash && stored.sceneRevision === document.scene_revision;
      if ((stored && stored.sceneRevision !== document.scene_revision) || (current && current.sceneRevision !== document.scene_revision && sourceDirty(current))) {
        await prepareProjectSourceHistorySwitch(projectId);
      }
      const text = document.content.replace(/\r\n/g, '\n');
      const file: SourceFile = {
        projectId, slideId, kind, phase: 'ready', sceneRevision: document.scene_revision,
        baseSourceHash: document.source_hash, baseContentHash: document.content_hash, baseText: text,
        draftText: sameCurrent ? current.draftText : sameStored ? stored.draftText : text,
        editGeneration: sameCurrent ? current.editGeneration : 0,
        resetVersion: sameCurrent ? current.resetVersion : (current?.resetVersion ?? 0) + 1,
        writable: document.writable, readonlyReason: document.readonly_reason,
        draftPersistence: sameCurrent ? current.draftPersistence : 'saved',
        selectionAnchor: sameCurrent ? current.selectionAnchor : sameStored ? stored.selectionAnchor : undefined,
        scrollTop: sameCurrent ? current.scrollTop : sameStored ? stored.scrollTop : undefined,
      };
      set((state) => ({ files: { ...state.files, [key]: file } }));
      if (stored && !sameStored) {
        await enqueueSourceDraft(stored.id, null);
        set((state) => { const drafts = { ...state.drafts }; delete drafts[stored.id]; return { drafts }; });
        if (current?.baseSourceHash && current.sceneRevision === document.scene_revision) showGlobalWarning('文件已更新，已加载最新内容');
      }
    } catch (error) {
      if (readTokens.get(key) !== token) return;
      const missing = error instanceof APIError && error.code === 'SOURCE_NOT_FOUND';
      const previous = get().files[key];
      set((state) => ({ files: { ...state.files, [key]: { ...previous, phase: missing ? 'missing' : 'error', writable: false, error: message(error) } } }));
      if (missing) {
        const id = sourceDraftId(projectId, slideId, kind);
        await enqueueSourceDraft(id, null);
        set((state) => { const drafts = { ...state.drafts }; delete drafts[id]; return { drafts }; });
      }
    }
  },
  setDraft: (key, text, selectionAnchor, scrollTop, expectedResetVersion) => {
    const file = get().files[key];
    if (!file || file.phase !== 'ready' || !file.writable || (expectedResetVersion !== undefined && file.resetVersion !== expectedResetVersion)) return;
    if (file.draftText === text && file.selectionAnchor === selectionAnchor && file.scrollTop === scrollTop) return;
    const textChanged = file.draftText !== text;
    const next = { ...file, draftText: text, editGeneration: file.editGeneration + (textChanged ? 1 : 0), selectionAnchor, scrollTop,
      draftPersistence: textChanged || sourceDirty(file) ? 'pending' as const : file.draftPersistence,
      diagnostics: textChanged ? undefined : file.diagnostics, error: textChanged ? undefined : file.error };
    set((state) => ({ files: { ...state.files, [key]: next } }));
    if (!textChanged && !sourceDirty(next)) return;
    const timer = persistenceTimers.get(key);
    if (timer) window.clearTimeout(timer);
    persistenceTimers.set(key, window.setTimeout(() => { void get().flush(key); }, 300));
  },
  flush: async (key) => {
    const keys = key ? [key] : Object.keys(get().files);
    for (const item of keys) {
      const timer = persistenceTimers.get(item);
      if (timer) { window.clearTimeout(timer); persistenceTimers.delete(item); }
      const file = get().files[item];
      if (!file?.baseSourceHash) continue;
      const record = draftRecord(file);
      try {
        await enqueueSourceDraft(record.id, sourceDirty(file) ? record : null);
        const current = get().files[item];
        if (current?.resetVersion !== file.resetVersion || current.baseSourceHash !== file.baseSourceHash || current.editGeneration !== file.editGeneration || current.draftText !== file.draftText) continue;
        if (sourceDirty(file)) set((state) => ({ drafts: { ...state.drafts, [record.id]: record } }));
        else set((state) => { const drafts = { ...state.drafts }; delete drafts[record.id]; return { drafts }; });
        set((state) => ({ files: { ...state.files, [item]: { ...state.files[item], draftPersistence: 'saved' } } }));
      } catch {
        set((state) => ({ files: { ...state.files, [item]: { ...state.files[item], draftPersistence: 'error' } } }));
      }
    }
    await flushSourceDrafts();
  },
  save: async (key) => {
    const file = get().files[key];
    if (!file || file.phase !== 'ready' || !file.writable || !file.baseSourceHash) return false;
    const { projectId, slideId, kind, draftText, baseSourceHash, sceneRevision, editGeneration } = file;
    set((state) => ({ files: { ...state.files, [key]: { ...file, phase: 'formatting', error: undefined, diagnostics: undefined } } }));
    await get().flush(key);
    let candidate: string | undefined;
    try {
      candidate = kind === 'spec' ? await formatStrictJSON(draftText) : await formatHTML(draftText);
      const current = get().files[key];
      if (current?.editGeneration !== editGeneration || current.sceneRevision !== sceneRevision || current.phase !== 'formatting') return false;
      set((state) => ({ files: { ...state.files, [key]: { ...state.files[key], phase: 'saving' } } }));
      const result = await slideSourcesApi.save(projectId, slideId, kind, candidate, baseSourceHash, sceneRevision);
      const after = get().files[key];
      if (!after || after.sceneRevision !== sceneRevision || after.editGeneration !== editGeneration) return false;
      const document: SlideSourceDocument = result.document;
      const text = document.content.replace(/\r\n/g, '\n');
      set((state) => ({ files: { ...state.files, [key]: {
        ...state.files[key], phase: 'ready', baseSourceHash: document.source_hash, baseContentHash: document.content_hash,
        baseText: text, draftText: text, writable: document.writable, readonlyReason: document.readonly_reason,
        draftPersistence: 'saved', error: undefined, diagnostics: undefined,
      } } }));
      const id = sourceDraftId(projectId, slideId, kind);
      set((state) => { const drafts = { ...state.drafts }; delete drafts[id]; return { drafts }; });
      try { await enqueueSourceDraft(id, null); }
      catch { showGlobalWarning('源文件已保存，但本地草稿清理失败；刷新前请重试保存'); }
      if (result.changed) {
        try { await useProjectStore.getState().loadProjectContent(projectId); }
        catch { showGlobalWarning('源文件已保存，预览刷新失败，请重试加载项目'); }
      }
      showGlobalSuccess(result.changed ? '源文件已保存' : '文件内容未变化');
      return true;
    } catch (error) {
      const current = get().files[key];
      if (error instanceof APIError && (error.code === 'CONTENT_CONFLICT' || error.code === 'SOURCE_SCENE_CHANGED')) {
        if (error.code === 'SOURCE_SCENE_CHANGED') {
          try { await prepareProjectSourceHistorySwitch(projectId); }
          catch (backupError) {
            if (current) set((state) => ({ files: { ...state.files, [key]: { ...state.files[key], phase: 'error', writable: false, error: message(backupError) } } }));
            return false;
          }
        }
        if (current) set((state) => ({ files: { ...state.files, [key]: { ...state.files[key], phase: 'reloading' } } }));
        await get().load(projectId, slideId, kind, true);
      } else if (error instanceof NetworkError || error instanceof RequestTimeoutError || (error instanceof APIError && error.status >= 500)) {
        // The server may have replaced the file before the response failed.
        // Keep the old draft read-only until the authoritative source is read.
        if (current) set((state) => ({ files: { ...state.files, [key]: { ...state.files[key], phase: 'reloading', error: '正在确认保存结果…' } } }));
        await get().load(projectId, slideId, kind, true);
        const checked = get().files[key];
        if (checked?.phase === 'ready' && checked.baseSourceHash === baseSourceHash) {
          set((state) => ({ files: { ...state.files, [key]: { ...state.files[key], error: '保存结果未确认，正式文件仍为原版本；请检查后重试' } } }));
        }
      } else if (current) {
        const diagnostic = error instanceof SourceFormatError && error.from !== undefined ? [{ from: error.from, to: error.to ?? error.from + 1, message: error.message, severity: 'error' as const }] : error instanceof APIError ? serverDiagnostic(error, draftText, candidate) : undefined;
        const busy = error instanceof APIError && ['RUN_ACTIVE', 'GIT_COMMIT_ACTIVE', 'HISTORY_BUSY', 'HISTORY_REVISION_CONFLICT'].includes(error.code ?? '');
        set((state) => ({ files: { ...state.files, [key]: { ...state.files[key], phase: 'ready', error: message(error), diagnostics: diagnostic,
          writable: busy ? false : state.files[key].writable, readonlyReason: busy ? error.code ?? null : state.files[key].readonlyReason } } }));
      }
      showGlobalWarning(message(error));
      return false;
    }
  },
  saveAll: async (projectId) => {
    await get().ensureIndex();
    await get().flush();
    const rows = Object.values(get().drafts).filter((item) => item.projectId === projectId && item.draftText !== item.baseText);
    const order = useProjectStore.getState().contentByProjectId[projectId]?.outline;
    const slideIds = order ? order.sections.flatMap((section) => [...section.slides.map((item) => item.slide_id), ...section.subsections.flatMap((sub) => sub.slides.map((item) => item.slide_id))]) : [];
    const position = (id: string) => { const found = slideIds.indexOf(id); return found < 0 ? Number.MAX_SAFE_INTEGER : found; };
    rows.sort((a, b) => (position(a.slideId) - position(b.slideId)) || (a.kind === b.kind ? 0 : a.kind === 'spec' ? -1 : 1));
    for (const row of rows) {
      const key = sourceKey(projectId, row.slideId, row.kind);
      await get().load(projectId, row.slideId, row.kind);
      if (!(await get().save(key))) {
        useDeckStore.getState().setCurrentSlideId(row.slideId);
        useDeckStore.getState().setGlobalView(row.kind === 'spec' ? 'outline' : 'html');
        useDeckStore.getState().setContentMode('source');
        return false;
      }
    }
    return true;
  },
  refreshProject: async (projectId) => {
    if (!get().indexReady) return;
    const files = Object.values(get().files).filter((file) => file.projectId === projectId && (file.phase === 'ready' || file.phase === 'error'));
    const ids = new Set(files.map((file) => sourceKey(file.projectId, file.slideId, file.kind)));
    const pending = Object.values(get().drafts).filter((draft) => draft.projectId === projectId && !ids.has(sourceKey(projectId, draft.slideId, draft.kind)));
    await Promise.all([...files.map((file) => get().load(file.projectId, file.slideId, file.kind, true)),
      ...pending.map((draft) => get().load(projectId, draft.slideId, draft.kind, true))]);
  },
}));

export function projectSourceDraftCount(projectId: string): number {
  const state = useSourceEditorStore.getState();
  const ids = new Set(Object.values(state.drafts).filter((draft) => draft.projectId === projectId && draft.draftText !== draft.baseText).map((draft) => draft.id));
  for (const file of Object.values(state.files)) {
    if (file.projectId === projectId && sourceDirty(file)) ids.add(sourceDraftId(file.projectId, file.slideId, file.kind));
  }
  return ids.size;
}

export class SourceDraftBlockedError extends Error { constructor(count: number) { super(`有 ${count} 个源文件尚未保存，请先保存`); } }

export async function prepareProjectSourceHistorySwitch(projectId: string) {
  const state = useSourceEditorStore.getState();
  await state.ensureIndex();
  await state.flush();
  const current = useSourceEditorStore.getState();
  for (const file of Object.values(current.files)) {
    if (file.projectId !== projectId || !sourceDirty(file)) continue;
    const stored = current.drafts[sourceDraftId(file.projectId, file.slideId, file.kind)];
    if (!stored || stored.draftText !== file.draftText || stored.sceneRevision !== file.sceneRevision) throw new Error('历史草稿暂存失败，请重试');
  }
  await backupProjectSourceDrafts(projectId);
}

export async function reconcileProjectSourceScene(projectId: string, sceneRevision: number) {
  const state = useSourceEditorStore.getState();
  await state.ensureIndex();
  await state.flush();
  const current = useSourceEditorStore.getState();
  const stale = Object.values(current.drafts).filter((draft) => draft.projectId === projectId && draft.sceneRevision !== sceneRevision);
  const staleFiles = Object.values(current.files).filter((file) => file.projectId === projectId && file.sceneRevision !== sceneRevision && file.baseSourceHash);
  if (staleFiles.some((file) => sourceDirty(file) && !current.drafts[sourceDraftId(file.projectId, file.slideId, file.kind)])) {
    throw new Error('历史草稿暂存失败，请重试');
  }
  if (!stale.length && !staleFiles.length) return;
  for (const scene of new Set(stale.map((draft) => draft.sceneRevision))) await backupProjectSourceDrafts(projectId, scene);
  await Promise.all(stale.map((draft) => enqueueSourceDraft(draft.id, null)));
  useSourceEditorStore.setState((current) => {
    const drafts = { ...current.drafts };
    for (const draft of stale) delete drafts[draft.id];
    return { drafts };
  });
  await Promise.all(staleFiles.map((file) => useSourceEditorStore.getState().load(file.projectId, file.slideId, file.kind, true)));
}

export async function assertProjectSourcesSaved(projectId: string) {
  const state = useSourceEditorStore.getState();
  await state.ensureIndex();
  await state.flush();
  const history = await projectHistoryApi.state(projectId);
  await reconcileProjectSourceScene(projectId, history.scene_revision);
  const count = projectSourceDraftCount(projectId);
  const pending = Object.values(useSourceEditorStore.getState().files).some((file) => file.projectId === projectId && ['formatting', 'saving', 'reloading', 'loading'].includes(file.phase));
  if (count || pending) throw new SourceDraftBlockedError(count || 1);
}
