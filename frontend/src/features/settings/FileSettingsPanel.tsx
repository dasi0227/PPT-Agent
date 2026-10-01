import { useEffect, useRef, useState } from 'react';
import { Plus, Trash2 } from 'lucide-react';
import { filesApi, type FileOpenMethod, type FileOpenWith, type FileSettings } from '../../api/files';
import { Select } from '../../components/ui/select';
import { IconButton } from '../../components/ui/primitives';
import { FileApplicationIcon } from '../../components/ui/FileApplicationIcon';
import { useFileSettingsStore } from '../../stores/fileSettingsStore';

type MethodKey = 'default' | 'json' | 'html';
const presets = [{ id: 'finder', name: '访达' }, { id: 'textedit', name: '文本编辑' }, { id: 'vscode', name: 'VS Code' }] as const;
const methodValue = (method: FileOpenMethod) => method.open_with === 'custom' ? `app:${method.custom_app_path}` : method.open_with;

export function FileSettingsPanel({ refreshKey, onSavingChange }: {
  refreshKey: number; onSavingChange: (saving: boolean) => void;
}) {
  const value = useFileSettingsStore(state => state.value);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const inFlight = useRef(false);
  useEffect(() => {
    let active = true;
    setLoading(true); setError('');
    void useFileSettingsStore.getState().load()
      .catch(cause => { if (active) setError(cause instanceof Error ? cause.message : '文件设置读取失败'); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [refreshKey]);
  const update = async (prepare: (base: FileSettings) => Promise<FileSettings | null>) => {
    if (!value || inFlight.current) return;
    inFlight.current = true; setBusy(true); onSavingChange(true); setError('');
    try {
      const edit = await prepare({ default: { ...value.default }, json: { ...value.json }, html: { ...value.html },
        custom_apps: value.custom_apps, revision: value.revision });
      if (edit) await useFileSettingsStore.getState().save(edit);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '文件设置保存失败，原设置仍然有效');
    } finally { inFlight.current = false; setBusy(false); onSavingChange(false); }
  };
  const add = () => update(async base => {
    const { application } = await filesApi.pickApplication();
    if (!application || application.builtin || base.custom_apps.some(app => app.path === application.path)) return null;
    return { ...base, custom_apps: [...base.custom_apps, application] };
  });
  const remove = (path: string) => update(async base => {
    const edit = { ...base, custom_apps: base.custom_apps.filter(app => app.path !== path) };
    for (const key of ['default', 'json', 'html'] as const) {
      if (edit[key].open_with === 'custom' && edit[key].custom_app_path === path) {
        edit[key] = { open_with: key === 'default' ? 'system' : 'inherit', custom_app_path: '' };
      }
    }
    return edit;
  });
  const change = (key: MethodKey, choice: string) => update(async base => ({ ...base,
    [key]: { open_with: choice.startsWith('app:') ? 'custom' : choice as FileOpenWith,
      custom_app_path: choice.startsWith('app:') ? choice.slice(4) : '' },
  }));
  const options = [
    { value: 'system', label: '系统默认应用', icon: <FileApplicationIcon size={20} /> },
    ...presets.map(app => ({ value: app.id, label: app.name, icon: <FileApplicationIcon application={app.id} size={20} /> })),
    ...(value?.custom_apps ?? []).map(app => ({ value: `app:${app.path}`, label: app.name, icon: <FileApplicationIcon application={app.name} size={20} /> })),
  ];
  const inherit = { value: 'inherit', label: '跟随默认', icon: <svg width={20} height={20} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round" strokeLinejoin="round" className="shrink-0 text-text-500" aria-hidden="true">
    <path d="M7 3v12a4 4 0 0 0 4 4h9m-4-4 4 4-4 4M3 7l4-4 4 4" />
  </svg> };
  return <>
    <div className="settings-heading"><h1>文件打开</h1></div>
    {error && <p role="alert" className="mb-5 text-sm text-danger">{error}，可点击顶部刷新后重试。</p>}
    {loading ? <p className="py-16 text-sm text-text-600">正在读取文件设置…</p> : value && <div className="settings-routing settings-file-open">
      <h2>文件类型</h2>
      <div className="settings-group">
        {([['default', '默认方式'], ['json', 'JSON 文件'], ['html', 'HTML 文件']] as const).map(([key, label]) => <div className="settings-row" key={key}>
          <h3>{label}</h3>
          <Select aria-label={label} className="settings-value" value={methodValue(value[key])} options={key === 'default' ? options : [inherit, ...options]}
            disabled={busy || !value.supported} onValueChange={choice => void change(key, choice)} />
        </div>)}
      </div>
      <div className="settings-apps-heading">
        <h2>打开应用</h2>
        <button type="button" className="settings-primary ui-interactive settings-add-app inline-flex shrink-0 items-center gap-1.5"
          disabled={busy || !value.supported} aria-busy={busy} onClick={() => void add()}><Plus size={15} aria-hidden="true" />添加应用</button>
      </div>
      <div className="settings-group">
        {presets.map(app => <div key={app.id} className="settings-row"><div className="settings-app-label"><FileApplicationIcon application={app.id} /><h3>{app.name}</h3></div></div>)}
        {value.custom_apps.map(app => <div key={app.path} className="settings-row"><div className="settings-app-label"><FileApplicationIcon application={app.name} /><h3>{app.name}</h3></div>
          <IconButton label={`删除 ${app.name}`} title="删除" className="text-danger ui-danger" disabled={busy || !value.supported} onClick={() => void remove(app.path)}><Trash2 size={16} aria-hidden="true" /></IconButton>
        </div>)}
      </div>
      {!value.supported && <p className="mt-4 text-sm text-text-600">当前仅支持在 macOS 本机打开文件。</p>}
    </div>}
  </>;
}
