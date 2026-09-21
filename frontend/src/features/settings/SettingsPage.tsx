import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useBlocker, useLocation, useNavigate } from 'react-router-dom';
import { Cpu, Eye, EyeOff, Home, Loader2, Plus, RefreshCw, Route, Trash2 } from 'lucide-react';
import { settingsApi, SIDE_PURPOSES, type ModelSettings, type ModelConfig, type SidePurpose } from '../../api/settings';
import { ConfirmModal } from '../../components/ui/modal-confirm';
import { IconButton } from '../../components/ui/primitives';
import { useComposerStore } from '../../stores/composerStore';
import { useProjectStore } from '../../stores/projectStore';
import { showGlobalSuccess, showGlobalWarning } from '../../stores/toastStore';
import { homeRoute, projectRoute } from '../workspace/routes';
import { cn } from '../../lib/utils';
import './settings.css';

type DraftModel = ModelConfig & { id: string; previous_name?: string; originalProvider: string; key: string };
type Draft = Omit<ModelSettings, 'llm'> & { llm: DraftModel[] };
type Editable = Pick<DraftModel, 'name' | 'provider' | 'model' | 'key'>;
const providerNames: Record<string, string> = { openai: 'OpenAI', deepseek: 'DeepSeek', kimi: 'Kimi' };
const purposeLabels: Record<SidePurpose, [string, string]> = {
  rename: ['会话命名', '自动命名与立即命名'], compact: ['上下文压缩', '自动压缩与手动压缩'],
  commit: ['提交说明', '生成 Git 提交标题与摘要'], polish: ['输入润色', '整理并润色当前输入'],
  handoff: ['交接内容', '生成交接说明及后续修订'], kickoff: ['启动说明', '生成开发启动说明及后续修订'],
};
function makeDraft(value: ModelSettings): Draft {
  return { ...value, llm: value.llm.map((row) => ({ ...row, id: crypto.randomUUID(), previous_name: row.name, originalProvider: row.provider, key: '' })) };
}
function validation(row: DraftModel, rows: DraftModel[]): string {
  if (!row.name.trim()) return '请填写配置名称。';
  if ([...row.name.trim()].length > 80) return '名称最多为 80 个字符。';
  if (rows.some((other) => other.id !== row.id && other.name.trim() === row.name.trim())) return '配置名称已被使用。';
  if (!row.provider || !row.model.trim()) return '请选择供应商并填写模型标识。';
  if (/\p{Cc}/u.test(row.name + row.model + row.key)) return '请使用单行文本，不要包含控制字符。';
  if (!row.key.trim() && (!row.has_key || row.provider !== row.originalProvider)) return '请填写 API key；更换供应商需要重新填写。';
  return '';
}
function mergeEdits(draft: Draft, edits: Record<string, Editable>): Draft {
  const names = new Map<string, string>();
  const llm = draft.llm.map((row) => {
    const edit = edits[row.id];
    if (!edit) return row;
    const next = { ...row, ...edit, name: edit.name.trim(), model: edit.model.trim(), key: edit.key.trim() };
    if (row.name) names.set(row.name, next.name);
    return next;
  });
  const replace = (name: string | null) => name ? names.get(name) ?? name : '';
  return { ...draft, llm,
    main_road: { default: replace(draft.main_road.default), fallback: replace(draft.main_road.fallback) },
    side_road: { ...draft.side_road, ...Object.fromEntries(Object.entries(draft.side_road).map(([key, value]) => [key, replace(value)])) },
  };
}

function ModelCard({ row, edit, open, providers, busy, error, onOpen, onEdit, onSave, onDelete }: {
  row: DraftModel; edit: Editable; open: boolean; providers: string[]; busy: boolean; error?: string;
  onOpen: () => void; onEdit: (edit: Editable) => void; onSave: () => void; onDelete: () => void;
}) {
  const [showKey, setShowKey] = useState(false);
  const front = useRef<HTMLButtonElement>(null);
  const back = useRef<HTMLFormElement>(null);
  useEffect(() => {
    const form = back.current;
    if (!open) {
      if (form?.contains(document.activeElement)) front.current?.focus();
      if (form) form.inert = true;
      setShowKey(false);
      return;
    }
    if (form) { form.inert = false; form.focus(); }
    const timer = window.setTimeout(() => {
      if (document.activeElement === form) form?.querySelector('select')?.focus();
    }, 350);
    return () => window.clearTimeout(timer);
  }, [open]);
  const update = (field: keyof Editable, value: string) => onEdit({ ...edit, [field]: value });
  return (
    <article className="settings-card" data-model-card={row.id} data-open={open}>
      <div className="settings-card-turn">
        <button ref={front} type="button" className="settings-card-face settings-card-front" disabled={busy}
          aria-expanded={open} aria-controls={`model-${row.id}`} tabIndex={open ? -1 : 0} aria-hidden={open}
          aria-label={`编辑 ${row.name || '新模型'}`} onClick={onOpen}>
          <img src={`/model-logos/${row.provider || 'openai'}.svg`} alt="" width={60} height={60} />
          <span>{row.name || '新模型'}</span>
        </button>
        <form ref={back} id={`model-${row.id}`} className="settings-card-face settings-card-back" tabIndex={-1} aria-hidden={!open}
          aria-label={`编辑 ${row.name || '新模型'}`} aria-describedby={error ? `error-${row.id}` : undefined}
          onSubmit={(event) => { event.preventDefault(); onSave(); }} autoComplete="off" noValidate>
          <fieldset disabled={busy}>
            <label><span>provider</span><select value={edit.provider} onChange={(event) => update('provider', event.target.value)}>
              {providers.map((provider) => <option key={provider} value={provider}>{providerNames[provider] ?? provider}</option>)}
            </select></label>
            <label><span>name</span><input value={edit.name} onChange={(event) => update('name', event.target.value)} maxLength={80} placeholder="配置名称" /></label>
            <label><span>model</span><input value={edit.model} onChange={(event) => update('model', event.target.value)} spellCheck={false} autoCapitalize="off" placeholder="模型标识" /></label>
            <label><span>key</span><span className="settings-key">
              <input type={showKey ? 'text' : 'password'} value={edit.key} onChange={(event) => update('key', event.target.value)}
                spellCheck={false} autoCapitalize="off" placeholder={row.has_key && edit.provider === row.originalProvider ? '已配置，留空保留' : '输入 API key'} autoComplete="new-password" />
              <button type="button" onClick={() => setShowKey(!showKey)} aria-label={showKey ? '隐藏密钥' : '显示密钥'} aria-pressed={showKey}>
                {showKey ? <EyeOff size={15} /> : <Eye size={15} />}
              </button>
            </span></label>
          </fieldset>
          <div className="settings-card-actions">
            <p id={`error-${row.id}`} className="settings-card-error" role="alert">{error}</p>
            <div className="flex items-center gap-2">
              <button type="submit" className="settings-primary flex-1" disabled={busy}>保存</button>
              <IconButton label="删除模型" type="button" title="删除模型" aria-label={`删除 ${row.name || '新模型'}`} onClick={onDelete} disabled={busy} className="text-danger hover:bg-danger-soft"><Trash2 size={16} /></IconButton>
            </div>
          </div>
        </form>
      </div>
    </article>
  );
}

export function SettingsPage() {
  const location = useLocation();
  const navigate = useNavigate();
  const activeProjectId = useProjectStore((state) => state.activeProjectId);
  const returnTo = typeof location.state?.returnTo === 'string' && !location.state.returnTo.startsWith('/settings')
    ? location.state.returnTo : activeProjectId ? projectRoute(activeProjectId) : homeRoute;
  const [section, setSection] = useState<'models' | 'routing'>('models');
  const [draft, setDraft] = useState<Draft | null>(null);
  const [baseline, setBaseline] = useState('');
  const [edits, setEdits] = useState<Record<string, Editable>>({});
  const [openCard, setOpenCard] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [cardErrors, setCardErrors] = useState<Record<string, string>>({});
  const [deleting, setDeleting] = useState<DraftModel | null>(null);
  const [reloadPrompt, setReloadPrompt] = useState(false);
  const grid = useRef<HTMLDivElement>(null);
  const leaving = useRef(false);
  const dirty = !!draft && (JSON.stringify(draft) !== baseline || Object.keys(edits).length > 0);
  const blocker = useBlocker(({ currentLocation, nextLocation }) => (dirty || saving) && currentLocation.pathname !== nextLocation.pathname);
  useEffect(() => {
    if (blocker.state === 'blocked' && !dirty && !saving) blocker.proceed();
  }, [blocker, dirty, saving]);
  const load = useCallback(async () => {
    setLoading(true); setError('');
    try {
      const value = makeDraft(await settingsApi.reload());
      setDraft(value); setBaseline(JSON.stringify(value)); setEdits({}); setCardErrors({}); setOpenCard(null);
      useComposerStore.getState().reconcileModels(value.llm.map((row) => row.name), value.main_road.default);
      window.dispatchEvent(new Event('model-settings-saved'));
    } catch (cause) { setError(cause instanceof Error ? cause.message : '模型设置加载失败。'); }
    finally { setLoading(false); }
  }, []);
  useEffect(() => { void load(); }, [load]);
  useEffect(() => {
    const unload = (event: BeforeUnloadEvent) => { if (dirty || saving) { event.preventDefault(); event.returnValue = ''; } };
    window.addEventListener('beforeunload', unload);
    return () => window.removeEventListener('beforeunload', unload);
  }, [dirty, saving]);
  useEffect(() => {
    const close = (event: PointerEvent) => {
      const target = event.target as Element;
      if (openCard && target.closest('[data-model-card]')?.getAttribute('data-model-card') !== openCard) setOpenCard(null);
    };
    const escape = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && openCard) {
        const button = grid.current?.querySelector<HTMLButtonElement>(`[data-model-card="${openCard}"] .settings-card-front`);
        setOpenCard(null); button?.focus();
      }
    };
    document.addEventListener('pointerdown', close); document.addEventListener('keydown', escape);
    return () => { document.removeEventListener('pointerdown', close); document.removeEventListener('keydown', escape); };
  }, [openCard]);
  const effective = useMemo(() => draft ? mergeEdits(draft, edits) : null, [draft, edits]);

  function saveCard(row: DraftModel) {
    if (!draft) return;
    const next = mergeEdits(draft, edits[row.id] ? { [row.id]: edits[row.id] } : {});
    const saved = next.llm.find((item) => item.id === row.id)!;
    const message = validation(saved, next.llm);
    if (message) { setCardErrors((current) => ({ ...current, [row.id]: message })); return; }
    setDraft(next); setEdits((current) => { const value = { ...current }; delete value[row.id]; return value; });
    setCardErrors((current) => ({ ...current, [row.id]: '' })); setOpenCard(null);
    showGlobalSuccess('已保存到草稿，点击“保存全部”后生效。');
  }
  async function saveAll() {
    if (!effective || !draft || saving) return;
    setError('');
    for (const row of effective.llm) {
      const message = validation(row, effective.llm);
      if (message) { setCardErrors({ [row.id]: message }); setSection('models'); setOpenCard(row.id); return; }
    }
    if (!effective.llm.length) { setError('至少保留一个模型。'); return; }
    const names = new Set(effective.llm.map((row) => row.name));
    if (!names.has(effective.main_road.default) || !names.has(effective.side_road.default)) {
      setError('请选择主路和旁路的默认模型。'); setSection('routing'); return;
    }
    if ([...Object.values(effective.main_road), ...Object.values(effective.side_road)].some((name) => name && !names.has(name))) {
      setError('模型分配包含已失效的名称，请重新选择。'); setSection('routing'); return;
    }
    setSaving(true);
    try {
      const result = await settingsApi.save({
        revision: effective.revision, main_road: effective.main_road, side_road: effective.side_road,
        llm: effective.llm.map((row) => ({ name: row.name, provider: row.provider, model: row.model,
          ...(row.previous_name ? { previous_name: row.previous_name } : {}), ...(row.key.trim() ? { key: row.key.trim() } : {}),
        })),
      });
      const selected = useComposerStore.getState();
      const renamed = effective.llm.find((row) => row.previous_name === selected.modelProfileName);
      if (selected.modelSelectionExplicit && renamed && renamed.name !== renamed.previous_name) selected.setModelProfileName(renamed.name);
      useComposerStore.getState().reconcileModels(result.llm.map((row) => row.name), result.main_road.default);
      const next = makeDraft(result);
      setDraft(next); setBaseline(JSON.stringify(next)); setEdits({}); setOpenCard(null); setCardErrors({});
      window.dispatchEvent(new Event('model-settings-saved'));
      showGlobalSuccess('设置已保存，新任务将使用新配置。');
    } catch (cause) { setError(cause instanceof Error ? cause.message : '保存失败，草稿已保留。'); }
    finally { setSaving(false); }
  }
  function addModel() {
    if (!draft) return;
    const row: DraftModel = { id: crypto.randomUUID(), name: '', provider: draft.providers[0], model: '', key: '', has_key: false, originalProvider: '' };
    setDraft({ ...draft, llm: [...draft.llm, row] }); setOpenCard(row.id);
  }
  function removeModel(row: DraftModel) {
    if (!draft || !effective) return;
    const name = effective.llm.find((item) => item.id === row.id)?.name;
    if (name && [...Object.values(effective.main_road), ...Object.values(effective.side_road)].includes(name)) {
      showGlobalWarning('此模型正在被使用，请先到“模型分配”解除引用。'); return;
    }
    if (draft.llm.length === 1) { showGlobalWarning('至少保留一个模型。'); return; }
    setDeleting(row);
  }
  const modelSelect = (value: string | null, change: (value: string) => void, label: string, empty?: string) => (
    <select className="settings-model-select" aria-label={label} value={value ?? ''} onChange={(event) => change(event.target.value)} disabled={saving}>
      {empty ? <option value="">{empty}</option> : !value ? <option value="" disabled>请选择模型</option> : null}
      {draft?.llm.filter((row) => row.name).map((row) => <option key={row.id} value={row.name}>{row.name}</option>)}
    </select>
  );
  return (
    <main className="model-settings flex h-[100dvh] min-h-0 flex-col overflow-hidden bg-workspace text-text-900">
      <header className="flex h-12 shrink-0 items-center border-b border-border-strong bg-surface px-2">
        <button type="button" onClick={() => navigate(returnTo)} className="flex items-center gap-2 rounded-md px-2 font-bold hover:bg-panel-muted" aria-label="返回项目">
          <img src="/logo.jpg" alt="" className="h-10 w-10 rounded-sm object-cover" /><span className="hidden sm:inline">Dasi PPT Agent</span>
        </button>
        <span className="mx-2 h-5 w-px bg-border" aria-hidden="true" /><span className="flex-1 font-bold">设置</span>
        <IconButton label="返回" type="button" title="返回" aria-label="返回" onClick={() => navigate(returnTo)} disabled={saving}><Home size={16} /></IconButton>
        <IconButton label="刷新设置" type="button" title="刷新设置" disabled={loading || saving} onClick={() => dirty ? setReloadPrompt(true) : void load()}><RefreshCw size={16} className={loading ? 'animate-spin motion-reduce:animate-none' : undefined} /></IconButton>
      </header>
      <div className="flex min-h-0 flex-1 flex-col md:flex-row">
        <nav className="flex shrink-0 gap-1 border-b border-border bg-panel p-3 md:w-52 md:flex-col md:border-b-0 md:border-r" aria-label="设置栏目">
          {([['models', '模型配置', Cpu], ['routing', '模型分配', Route]] as const).map(([id, label, Icon]) => (
            <button key={id} type="button" aria-current={section === id ? 'page' : undefined} onClick={() => { setSection(id); setOpenCard(null); }}
              className={cn('flex h-10 flex-1 items-center gap-2.5 rounded-lg px-3 text-sm md:flex-none', section === id ? 'bg-accent-soft font-semibold text-accent' : 'text-text-600 hover:bg-panel-muted')}><Icon size={17} />{label}</button>
          ))}
        </nav>
        <section className="min-h-0 min-w-0 flex-1 overflow-y-auto">
          <div className="settings-content">
            <div className="settings-heading">
              <div><h1>{section === 'models' ? '模型配置' : '模型分配'}</h1><p>{section === 'models' ? '点击卡片编辑模型，保存全部后生效。' : '为主路与各项旁路选择模型。'}</p></div>
              <div className="flex shrink-0 items-center gap-2">
                <button type="button" className="settings-primary flex items-center gap-2" disabled={!dirty || saving || loading} onClick={() => void saveAll()}>{saving && <Loader2 size={14} className="animate-spin" />}{saving ? '保存中…' : '保存全部'}</button>
              </div>
            </div>
            {error && <div className="mb-5 rounded-lg bg-danger-soft px-4 py-3 text-sm text-danger" role="alert">{error}</div>}
            {loading ? <div className="flex items-center gap-2 py-16 text-sm text-text-600"><Loader2 size={17} className="animate-spin" />正在读取设置…</div> : draft && (
              section === 'models' ? <>
                <div className="mb-4 flex justify-end"><button type="button" className="flex items-center gap-1.5 rounded-md px-2 py-1.5 text-sm text-accent hover:bg-accent-soft" disabled={saving} onClick={addModel}><Plus size={16} />添加模型</button></div>
                <div ref={grid} className="settings-grid">
                  {draft.llm.map((row) => <ModelCard key={row.id} row={row} edit={edits[row.id] ?? row} open={openCard === row.id} providers={draft.providers} busy={saving} error={cardErrors[row.id]}
                    onOpen={() => setOpenCard(row.id)} onEdit={(edit) => { setEdits((current) => ({ ...current, [row.id]: edit })); setCardErrors((current) => ({ ...current, [row.id]: '' })); }}
                    onSave={() => saveCard(row)} onDelete={() => removeModel(row)} />)}
                </div>
              </> : <div className="settings-routing">
                <h2>主路</h2>
                <div className="settings-group">
                  <div className="settings-row"><div><h3>默认模型</h3><p>聊天区没有手动选择时使用。</p></div>{modelSelect(draft.main_road.default, (value) => setDraft({ ...draft, main_road: { ...draft.main_road, default: value } }), '主路默认模型')}</div>
                  <div className="settings-row"><div><h3>备用模型</h3><p>原模型重试失败后接替，本轮持续使用。</p></div>{modelSelect(draft.main_road.fallback, (value) => setDraft({ ...draft, main_road: { ...draft.main_road, fallback: value } }), '主路备用模型', '不启用备用模型')}</div>
                </div>
                <h2>旁路</h2>
                <div className="settings-group">
                  <div className="settings-row"><div><h3>默认模型</h3><p>未单独指定模型的旁路使用此配置。</p></div>{modelSelect(draft.side_road.default, (value) => setDraft({ ...draft, side_road: { ...draft.side_road, default: value } }), '旁路默认模型')}</div>
                  <div className="settings-row"><div><h3>备用模型</h3><p>具体用途模型或默认模型重试失败后接替。</p></div>{modelSelect(draft.side_road.fallback, (value) => setDraft({ ...draft, side_road: { ...draft.side_road, fallback: value } }), '旁路备用模型', '不启用备用模型')}</div>
                  {SIDE_PURPOSES.map((purpose) => <div key={purpose} className="settings-row"><div><h3>{purposeLabels[purpose][0]}</h3><p>{purposeLabels[purpose][1]}</p></div>{modelSelect(draft.side_road[purpose], (value) => setDraft({ ...draft, side_road: { ...draft.side_road, [purpose]: value } }), `${purposeLabels[purpose][0]}模型`, '使用旁路默认模型')}</div>)}
                </div>
              </div>
            )}
          </div>
        </section>
      </div>
      <ConfirmModal open={blocker.state === 'blocked' && !saving} onOpenChange={(open) => { if (!open && !leaving.current && blocker.state === 'blocked') blocker.reset(); }} title="离开设置？" description="还有未保存的修改，离开后将丢弃这些修改。" confirmLabel="丢弃并离开" cancelLabel="继续编辑" onConfirm={() => { if (blocker.state === 'blocked') { leaving.current = true; blocker.proceed(); } }} />
      <ConfirmModal open={reloadPrompt} onOpenChange={setReloadPrompt} title="重新读取设置？" description="重新读取会丢弃当前页面的未保存修改。" confirmLabel="重新读取" cancelLabel="继续编辑" onConfirm={load} />
      <ConfirmModal open={!!deleting} onOpenChange={(open) => { if (!open) setDeleting(null); }} title="删除模型？" description={`从草稿中删除「${deleting?.name || '新模型'}」，保存全部后生效。`} confirmLabel="删除" onConfirm={() => {
        if (!draft || !deleting) return;
        setDraft({ ...draft, llm: draft.llm.filter((row) => row.id !== deleting.id) });
        setEdits((current) => { const next = { ...current }; delete next[deleting.id]; return next; }); setDeleting(null); setOpenCard(null);
      }} />
    </main>
  );
}
