import { useEffect, useState } from 'react';
import { Loader2 } from 'lucide-react';
import { useShortcutStore } from '../../stores/shortcutStore';
import { defaultBindings, bindingSignature, recordShortcut, shortcutCatalog, shortcutLabel, validateBindings, type ShortcutBinding, type ShortcutSettings } from '../../lib/shortcuts';
import { isMac } from '../../lib/platform';
import { showGlobalError } from '../../stores/toastStore';

export function ShortcutSettingsPanel({ refreshKey, onDirtyChange, onSavingChange }: {
  refreshKey: number;
  onDirtyChange: (dirty: boolean) => void;
  onSavingChange: (saving: boolean) => void;
}) {
  const revision = useShortcutStore(state => state.revision);
  const [draft, setDraft] = useState<ShortcutSettings | null>(null);
  const [baseline, setBaseline] = useState('');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [recording, setRecording] = useState<string | null>(null);
  const fingerprint = (value: ShortcutSettings) => JSON.stringify(shortcutCatalog.map(def => bindingSignature(value.bindings[def.id])));
  const dirty = !!draft && fingerprint(draft) !== baseline;
  const invalid = draft ? validateBindings(draft.bindings) : null;
  const stale = !!draft && revision !== draft.revision;

  useEffect(() => { onDirtyChange(dirty); }, [dirty, onDirtyChange]);
  useEffect(() => { onSavingChange(saving); }, [saving, onSavingChange]);
  useEffect(() => () => { onDirtyChange(false); onSavingChange(false); }, [onDirtyChange, onSavingChange]);
  useEffect(() => {
    let active = true;
    setLoading(true); setError('');
    void useShortcutStore.getState().load().then(() => {
      if (!active) return;
      const { revision, bindings } = useShortcutStore.getState();
      const next = { revision, bindings };
      setDraft(next); setBaseline(fingerprint(next));
    }).catch(cause => { if (active) setError(cause instanceof Error ? cause.message : '快捷键设置加载失败'); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [refreshKey]);

  const change = (id: string, binding: ShortcutBinding) => {
    setError('');
    setDraft(current => current ? { ...current, bindings: { ...current.bindings, [id]: binding } } : current);
  };
  useEffect(() => {
    if (!draft || !dirty || invalid || stale || saving || loading || error) return;
    // Wait for the current edit to settle; invalid drafts never replace the saved bindings.
    const timer = window.setTimeout(() => {
      setSaving(true);
      void useShortcutStore.getState().save(draft).then(value => {
        setDraft(value); setBaseline(fingerprint(value));
      }).catch(cause => {
        const message = cause instanceof Error ? cause.message : '快捷键保存失败';
        setError(message); showGlobalError(message);
      }).finally(() => setSaving(false));
    }, 250);
    return () => window.clearTimeout(timer);
  }, [draft, dirty, invalid, stale, saving, loading, error]);
  const heading = <div className="settings-heading">
    <h1>快捷键</h1>
    <div className="flex shrink-0 items-center gap-3">
      <span role="status" aria-live="polite" className="flex items-center gap-2 text-xs text-text-600">{saving && <><Loader2 size={14} className="animate-spin" />保存中…</>}</span>
      <button type="button" className="settings-primary ui-primary" disabled={loading || !draft || saving || stale || fingerprint(draft) === fingerprint({ revision: 0, bindings: defaultBindings })}
        onClick={() => { setError(''); setRecording(null); setDraft(current => current ? { ...current, bindings: defaultBindings } : current); }}>全部恢复至默认</button>
    </div>
  </div>;
  if (loading) return <>{heading}<div className="flex items-center gap-2 py-16 text-sm text-text-600"><Loader2 size={17} className="animate-spin" />正在读取快捷键…</div></>;
  if (!draft) return <>{heading}<p role="alert" className="text-sm text-danger">{error}</p></>;
  return (
    <>
    {heading}
    <div className="settings-routing">
      {(invalid || error || stale) && <p role="alert" className="mb-5 text-sm text-danger">{invalid || error || '配置已在其它窗口更新，请点击顶部刷新后重新编辑。'}</p>}
      {['输入触发符', '演示文稿', '输入框'].map(group => (
        <section key={group} className="mb-8">
          <h2>{group}</h2>
          <div className="settings-group">
            {shortcutCatalog.filter(def => def.group === group).map(def => {
              const binding = draft.bindings[def.id];
              return <div key={def.id} className="settings-row">
                <div><h3>{def.label}</h3></div>
                <input
                    aria-label={def.label}
                    value={recording === def.id && def.kind !== 'trigger' ? '按下组合键…' : shortcutLabel(binding)}
                    readOnly={def.kind !== 'trigger'}
                    disabled={saving}
                    maxLength={def.kind === 'trigger' ? 1 : undefined}
                    onChange={event => { if (def.kind === 'trigger') change(def.id, { trigger: event.target.value }); }}
                    onFocus={() => setRecording(def.id)}
                    onBlur={() => setRecording(null)}
                    onKeyDown={event => {
                      if (def.kind === 'trigger') return;
                      event.stopPropagation();
                      if (event.key === 'Tab') return;
                      event.preventDefault();
                      if (event.key === 'Escape') { event.currentTarget.blur(); return; }
                      if (event.nativeEvent.isComposing || ['Meta', 'Control', 'Alt', 'Shift'].includes(event.key)) return;
                      if (isMac() ? event.ctrlKey : event.metaKey) { setError('请使用 Command/Ctrl 或 Option/Alt 作为修饰键'); return; }
                      change(def.id, recordShortcut(event.nativeEvent));
                      event.currentTarget.blur();
                    }}
                    className="settings-value h-9 rounded-lg border border-border bg-panel px-3 text-center text-sm text-text-900 focus:bg-hover"
                  />
              </div>;
            })}
          </div>
        </section>
      ))}
    </div>
    </>
  );
}
