import { useEffect, useId, useRef, useState, type ReactNode } from 'react';
import type { LucideIcon } from 'lucide-react';
import type { PPTMutation, ProjectContentSnapshot, RestrictedPatch } from '../../api/types';
import { Button } from '../../components/ui/primitives';
import { TextField } from '../../components/ui/text-field';
import { useDeckStore } from '../../stores/deckStore';
import { useProjectStore } from '../../stores/projectStore';
import { showGlobalError, showGlobalSuccess, useToastStore } from '../../stores/toastStore';
import { DocumentCanvas } from './DocumentCanvas';
import { JSONPreview } from './JSONPreview';
import './management.css';

type Update<T> = (current: T) => T;
export interface TextEdit<T> {
  id: string; label: string; value: string; minLength?: number; allowEmpty?: boolean; maxLength: number; multiline?: boolean;
  update: (current: T, text: string) => T;
}
interface Draft<T> extends TextEdit<T> { base: T; hash?: string; scene?: string }
export interface ManagementController<T> {
  value: T; draft?: Draft<T>; saving: boolean; disabled: boolean; unavailable?: string;
  start: (edit: TextEdit<T>) => void;
  changeText: (text: string) => void;
  cancel: () => void;
  saveText: () => Promise<void>;
  commit: (update: Update<T>, undoable?: boolean) => Promise<void>;
  error?: string;
}

/** Only changed business fields are submitted. Optional fields are removed, never stored as null. */
export function changedFields<T extends object>(previous: T, next: T): RestrictedPatch[] {
  const before = previous as Record<string, unknown>;
  const after = next as Record<string, unknown>;
  return [...new Set([...Object.keys(before), ...Object.keys(after)])].flatMap((key): RestrictedPatch[] => {
    if (JSON.stringify(before[key]) === JSON.stringify(after[key])) return [];
    const path = `/${key.replace(/~/g, '~0').replace(/\//g, '~1')}`;
    if (after[key] === undefined) return [{ op: 'remove', path }];
    return [{ op: before[key] === undefined ? 'add' : 'replace', path, value: after[key] }];
  });
}

export function ManagementEditor<T>({ projectId, resourceKey, title, icon, value, hash, sceneRevision, blocked, mutation, children, notice, jsonAvailable = true }: {
  projectId?: string; resourceKey: string; title: string; icon: LucideIcon; value: T;
  hash?: string; sceneRevision?: string; blocked?: string;
  mutation: (next: T, previous: T) => PPTMutation;
  children: (editor: ManagementController<T>) => ReactNode;
  notice?: ReactNode; jsonAvailable?: boolean;
}) {
  const [draft, setDraft] = useState<Draft<T>>();
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string>();
  const showJSON = useDeckStore(state => state.contentMode === 'source');
  useEffect(() => {
    useDeckStore.getState().setSourceBlocked(Boolean(draft) || saving || !jsonAvailable);
    return () => useDeckStore.getState().setSourceBlocked(false);
  }, [draft, saving, jsonAvailable]);
  const undo = useRef<{ toastId: string; value: T; hash?: string; scene?: string; projectId?: string; resourceKey: string }>();
  const pending = useRef(false);
  const mounted = useRef(true);
  const returnTarget = useRef<HTMLElement | null>(null);
  const latest = useRef({ value, hash, sceneRevision, blocked });
  latest.current = { value, hash, sceneRevision, blocked };
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; clearUndo(); }; }, []);
  useEffect(() => {
    const previous = undo.current;
    if (previous && (previous.projectId !== projectId || previous.resourceKey !== resourceKey || blocked
      || previous.hash !== hash || previous.scene !== sceneRevision)) clearUndo();
  }, [projectId, resourceKey, hash, sceneRevision, blocked]);
  const stale = draft && (draft.hash !== hash || draft.scene !== sceneRevision);
  const unavailable = saving ? undefined : blocked ?? (stale ? '内容已更新，请取消后重新编辑。' : undefined);
  const disabled = !projectId || Boolean(blocked) || saving || Boolean(draft);

  function clearUndo() {
    if (undo.current) useToastStore.getState().removeToast(undo.current.toastId);
    undo.current = undefined;
  }

  function reportError(message: string) {
    setError(message);
    if (!draft) showGlobalError(message);
  }

  function focusTrigger() {
    // The button is mounted again after the inline editor closes.
    const id = returnTarget.current?.dataset.editId ?? returnTarget.current?.dataset.focusFallback;
    requestAnimationFrame(() => {
      if (!mounted.current) return;
      if (returnTarget.current?.isConnected) returnTarget.current.focus();
      else if (id) document.getElementById(id)?.focus();
    });
  }

  async function save(next: T, base: T, capturedHash: string | undefined, capturedScene: string | undefined, undoable = false): Promise<boolean> {
    if (!projectId || pending.current) return false;
    const current = latest.current;
    const snapshot = useProjectStore.getState().contentByProjectId[projectId];
    if (current.blocked) { reportError(current.blocked); return false; }
    // Check both props and the store; a queued render must not allow an old array to overwrite a new one.
    if (current.hash !== capturedHash || current.sceneRevision !== capturedScene
      || (snapshot && (snapshot.hashes[resourceKey] !== capturedHash || snapshot.scene_revision !== capturedScene))) {
      reportError('内容已更新，请取消后重新编辑。'); return false;
    }
    if (JSON.stringify(next) === JSON.stringify(base)) { setDraft(undefined); setError(undefined); focusTrigger(); return true; }
    clearUndo();
    pending.current = true; setSaving(true); setError(undefined);
    try {
      const result: ProjectContentSnapshot = await useProjectStore.getState().mutateProject(projectId, {
        ...mutation(next, base), expected_hash: capturedHash, expected_scene_revision: capturedScene,
      });
      if (!mounted.current) return true;
      setDraft(undefined);
      if (undoable) {
        const record = { toastId: '', value: base, hash: result.hashes[resourceKey], scene: result.scene_revision, projectId, resourceKey };
        record.toastId = showGlobalSuccess('已删除条目', { label: '撤销', onClick: () => {
          if (!mounted.current || pending.current || undo.current !== record) return;
          undo.current = undefined;
          void save(record.value, latest.current.value, record.hash, record.scene);
        } });
        undo.current = record;
      } else {
        showGlobalSuccess('已保存');
      }
      focusTrigger();
      return true;
    } catch (cause) {
      if (mounted.current) reportError(cause instanceof Error ? cause.message : '保存失败，请重试。');
      void useProjectStore.getState().loadProjectContent(projectId);
      return false;
    } finally {
      pending.current = false;
      if (mounted.current) setSaving(false);
    }
  }

  const controller: ManagementController<T> = {
    value: draft?.base ?? value, draft, saving, disabled, unavailable, error,
    start(edit) {
      if (disabled || pending.current) return;
      returnTarget.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
      setError(undefined); clearUndo();
      setDraft({ ...edit, base: structuredClone(value), hash, scene: sceneRevision });
    },
    changeText(text) { setDraft(current => current ? { ...current, value: text } : current); setError(undefined); },
    cancel() { if (pending.current) return; setDraft(undefined); setError(undefined); focusTrigger(); },
    async saveText() {
      if (!draft || unavailable || pending.current) return;
      const text = draft.value.trim();
      if ((!draft.allowEmpty || text !== '') && [...text].length < (draft.minLength ?? 0)) { setError(`请填写${draft.label}，至少 ${draft.minLength} 个字符。`); return; }
      if ([...text].length > draft.maxLength) { setError(`${draft.label}不能超过 ${draft.maxLength} 个字符。`); return; }
      await save(draft.update(draft.base, text), draft.base, draft.hash, draft.scene);
    },
    async commit(update, undoable = false) {
      if (disabled || pending.current) return;
      returnTarget.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
      const base = structuredClone(value);
      await save(update(base), base, hash, sceneRevision, undoable);
    },
  };

  if (showJSON && jsonAvailable) return <JSONPreview title={title} value={value} notice={notice} />;
  return <DocumentCanvas title={title} icon={icon} status={!saving && !notice ? blocked : undefined}>
    {notice}
    {unavailable && !blocked && <p className="management-error" role="alert">{unavailable}</p>}
    {children(controller)}
  </DocumentCanvas>;
}

export function InlineTextEditor<T>({ editor, hideLabel = false }: { editor: ManagementController<T>; hideLabel?: boolean }) {
  const errorId = useId();
  const draft = editor.draft;
  if (!draft) return null;
  return <form className="management-editor" noValidate onSubmit={event => { event.preventDefault(); void editor.saveText(); }}
    onKeyDown={event => {
      if (event.nativeEvent.isComposing) return;
      if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); editor.cancel(); }
      if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) { event.preventDefault(); void editor.saveText(); }
    }}>
    <fieldset disabled={editor.saving}>
      <TextField label={draft.label} hideLabel={hideLabel} value={draft.value} onChange={editor.changeText} minLength={draft.allowEmpty ? 0 : draft.minLength}
        maxLength={draft.maxLength} multiline={draft.multiline} autoFocus describedBy={errorId} invalid={Boolean(editor.error)} />
    </fieldset>
    <p id={errorId} className="management-error" role={editor.error ? 'alert' : undefined}>{editor.error}</p>
    <div className="management-editor-actions">
      <Button type="button" variant="ghost" className="management-cancel" disabled={editor.saving} onClick={editor.cancel}>取消</Button>
      <Button type="submit" variant="primary" disabled={editor.saving || Boolean(editor.unavailable)}>{editor.saving ? '保存中…' : '保存'}</Button>
    </div>
  </form>;
}
