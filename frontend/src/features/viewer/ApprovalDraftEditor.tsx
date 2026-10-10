import { ContentRequirementsIcon } from '../../components/ui/ContentRequirementsIcon';
import { useCallback, useEffect, useRef, useState } from 'react';
import { Palette } from 'lucide-react';
import type { Design, Manifest, Outline, ResourceEditApproval } from '../../api/types';
import { runsApi } from '../../api/runs';
import { Button } from '../../components/ui/primitives';
import { resourceApprovalKey, useResourceApprovalStore, type ActiveResourceApproval } from '../../stores/resourceApprovalStore';
import { useRunStore } from '../../stores/runStore';
import { useDeckStore } from '../../stores/deckStore';
import { DocumentCanvas } from './DocumentCanvas';
import { ManifestFields, DesignFields } from './AuthoringFields';
import type { ManagementController, TextEdit } from './ManagementEditor';
import { partLabel } from './semanticLabels';
import { OutlineEditor } from './OutlineEditor';
import { applyOutlineCommand } from './outlineEditing';
import { JSONPreview } from './JSONPreview';

type EditableResource = Manifest | Design | Outline;
type InlineDraft<T> = TextEdit<T> & { base: T };

function errorMessage(error: unknown) { return error instanceof Error ? error.message : '操作失败'; }

function useApprovalEditorState(record: ResourceEditApproval, dirty: boolean, busy: boolean) {
  const key = resourceApprovalKey(record.run_id, record.interaction_id);
  useEffect(() => { useResourceApprovalStore.getState().editorState(key, dirty, busy); }, [key, dirty, busy]);
  useEffect(() => () => { useResourceApprovalStore.getState().editorState(key, false, false); }, [key]);
}

function ApprovalActions({ busy, canRestore, onRestore, onSave }: {
  busy: boolean; canRestore: boolean; onRestore: () => void; onSave: () => void;
}) {
  return <div className="flex flex-wrap justify-end gap-2">
    <Button type="button" variant="ghost" disabled={busy || !canRestore} onClick={onRestore}>恢复</Button>
    <Button type="button" variant="ghost" disabled={busy} onClick={onSave}>{busy ? '保存中…' : '保存'}</Button>
  </div>;
}

function FieldApprovalEditor<T extends Manifest | Design>({ record, persist, saved }: {
  record: ResourceEditApproval; persist: (draft: EditableResource) => Promise<ResourceEditApproval>;
  saved: (revision: number) => void;
}) {
  const value = record.draft as T;
  const savedChanges = JSON.stringify(record.draft) !== JSON.stringify(record.proposal);
  const [inline, setInline] = useState<InlineDraft<T>>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const pending = useRef(false);
  const showJSON = useDeckStore(state => state.contentMode === 'source');
  useEffect(() => {
    useDeckStore.getState().setSourceBlocked(Boolean(inline) || busy);
    return () => useDeckStore.getState().setSourceBlocked(false);
  }, [inline, busy]);
  useApprovalEditorState(record, Boolean(inline), busy);
  const save = async (next: T) => {
    if (pending.current) return null;
    pending.current = true; setBusy(true); setError('');
    try { return await persist(next); }
    catch (cause) { setError(errorMessage(cause)); return null; }
    finally { pending.current = false; setBusy(false); }
  };
  const inlineValue = () => {
    if (!inline) return null;
    const text = inline.value.trim();
    if ([...text].length < (inline.minLength ?? 0) || [...text].length > inline.maxLength) {
      setError(`${inline.label}长度不符合要求`); return null;
    }
    return inline.update(inline.base, text);
  };
  const saveInline = async () => {
    const next = inlineValue();
    if (!next) return;
    const saved = await save(next);
    if (saved) setInline(undefined);
  };
  const restore = async () => {
    if (!record.proposal || busy || pending.current) return;
    if (savedChanges && !await save(structuredClone(record.proposal as T))) return;
    setInline(undefined); setError('');
  };
  const controller: ManagementController<T> = {
    value: inline?.base ?? value, draft: inline, saving: busy, disabled: busy || Boolean(inline), error,
    start(edit) { if (!busy) { setInline({ ...edit, base: structuredClone(value) }); setError(''); } },
    changeText(text) { setInline(current => current ? { ...current, value: text } : undefined); setError(''); },
    cancel() { setInline(undefined); setError(''); },
    saveText: saveInline,
    async commit(update) { if (!inline && !busy) await save(update(structuredClone(value))); },
  };
  const submit = async () => {
    let current = record;
    if (inline) {
      const next = inlineValue();
      if (!next) return;
      const saved = await save(next);
      if (!saved) return;
      current = saved;
      setInline(undefined);
    }
    setBusy(true);
    try { saved(current.revision); setBusy(false); }
    catch (cause) { setError(errorMessage(cause)); setBusy(false); }
  };
  const isManifest = record.resource === 'manifest';
  if (showJSON && !inline && !busy) return <JSONPreview title={partLabel(record.resource)} value={value} />;
  return <DocumentCanvas title={partLabel(record.resource)} icon={isManifest ? ContentRequirementsIcon : Palette}
    footer={<ApprovalActions busy={busy} canRestore={savedChanges || Boolean(inline)} onRestore={() => void restore()} onSave={() => void submit()} />}>
    {error && <p className="mb-3 text-xs text-danger" role="alert">{error}</p>}
    {isManifest ? <ManifestFields editor={controller as unknown as ManagementController<Manifest>} />
      : <DesignFields editor={controller as unknown as ManagementController<Design>} />}
  </DocumentCanvas>;
}

function OutlineApprovalEditor({ record, persist, saved }: {
  record: ResourceEditApproval; persist: (draft: EditableResource) => Promise<ResourceEditApproval>;
  saved: (revision: number) => void;
}) {
  const outline = record.draft as Outline;
  const key = resourceApprovalKey(record.run_id, record.interaction_id);
  const savedChanges = JSON.stringify(record.draft) !== JSON.stringify(record.proposal);
  const version = String(record.revision);
  const latestVersion = useRef(version);
  latestVersion.current = version;
  useEffect(() => () => { useResourceApprovalStore.getState().editorState(key, false, false); }, [key]);
  const editorState = useCallback((isEditing: boolean, isBusy: boolean) => {
    useResourceApprovalStore.getState().editorState(key, isEditing, isBusy);
  }, [key]);
  const checkVersion = (capturedVersion: string) => {
    if (latestVersion.current !== capturedVersion) throw new Error('草稿已更新，请取消后重新编辑。');
  };
  return <OutlineEditor value={outline} version={version} draftMode canRestore={savedChanges}
    onEditingChange={editorState} onDraftSaved={saved}
    commit={async (command, capturedVersion) => {
      checkVersion(capturedVersion);
      await persist(applyOutlineCommand(outline, command));
    }}
    saveDraft={async (next, capturedVersion) => {
      checkVersion(capturedVersion);
      const current = JSON.stringify(next) === JSON.stringify(record.draft) ? record : await persist(next);
      return current.revision;
    }}
    restoreDraft={async () => {
      if (!record.proposal) return;
      if (savedChanges) await persist(structuredClone(record.proposal as Outline));
    }} />;
}

export function ApprovalDraftEditor({ active }: { active: ActiveResourceApproval }) {
  const [record, setRecord] = useState<ResourceEditApproval | null>(null);
  const [error, setError] = useState('');
  const close = useResourceApprovalStore(state => state.close);
  useEffect(() => {
    if (record?.state === 'pending') return;
    useDeckStore.getState().setSourceBlocked(true);
    return () => useDeckStore.getState().setSourceBlocked(false);
  }, [record]);
  useEffect(() => {
    let live = true;
    setRecord(null); setError('');
    void runsApi.getResourceEditApproval(active.runId, active.interactionId)
      .then(next => { if (live) { setRecord(next); useRunStore.getState().syncResourceApprovalDraft(next); } })
      .catch(cause => { if (live) setError(errorMessage(cause)); });
    return () => { live = false; };
  }, [active.runId, active.interactionId]);
  const persist = async (draft: EditableResource) => {
    if (!record) throw new Error('草稿尚未加载');
    const next = await runsApi.updateResourceEditDraft(active.runId, active.interactionId, record.revision, draft);
    setRecord(next);
    useRunStore.getState().syncResourceApprovalDraft(next);
    return next;
  };
  const saved = useCallback((revision: number) => {
    useResourceApprovalStore.getState().saved(resourceApprovalKey(active.runId, active.interactionId), revision);
  }, [active.runId, active.interactionId]);
  if (!record) return <DocumentCanvas title={partLabel(active.resource)}>{error ? <p role="alert" className="text-danger">{error}</p> : <p className="text-text-600">加载中…</p>}</DocumentCanvas>;
  if (record.state !== 'pending') return <DocumentCanvas title={partLabel(active.resource)}><Button onClick={close}>返回</Button></DocumentCanvas>;
  if (active.resource === 'outline') return <OutlineApprovalEditor key={record.interaction_id} record={record} persist={persist} saved={saved} />;
  return <FieldApprovalEditor key={record.interaction_id} record={record} persist={persist} saved={saved} />;
}
