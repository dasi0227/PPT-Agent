import { useEffect, useRef, useState } from 'react';
import { Plus, Trash2 } from 'lucide-react';
import { filesApi, applicationName, type FileSettings } from '../../api/files';
import { Select } from '../../components/ui/select';
import { Button, IconButton } from '../../components/ui/primitives';
import { FileApplicationIcon } from '../../components/ui/FileApplicationIcon';
import { FollowDefaultIcon } from '../../components/ui/FollowDefaultIcon';
import { useFileSettingsStore } from '../../stores/fileSettingsStore';

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
      const edit = await prepare(value);
      if (edit) await useFileSettingsStore.getState().save(edit);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '文件设置保存失败，原设置仍然有效');
    } finally { inFlight.current = false; setBusy(false); onSavingChange(false); }
  };
  const add = () => update(async base => {
    const { path } = await filesApi.pickApplication();
    if (!path) return null;
    return { ...base, custom_apps: [...base.custom_apps, path] };
  });
  const remove = (path: string) => update(async base => ({ ...base, custom_apps: base.custom_apps.filter(app => app !== path) }));
  const change = (key: 'default_open' | 'json_open' | 'html_open', choice: string) => update(async base => ({ ...base, [key]: choice }));
  const options = [
    { value: 'system', label: '系统默认应用', icon: <FileApplicationIcon size={20} /> },
    ...(value?.custom_apps ?? []).map(path => ({ value: path, label: applicationName(path), icon: <FileApplicationIcon application={applicationName(path)} size={20} /> })),
  ];
  const inherit = { value: 'inherit', label: '跟随默认', icon: <FollowDefaultIcon /> };
  return <>
    <div className="settings-heading">
      <h1>文件打开</h1>
      <Button type="button" variant="secondary" icon={Plus} className="shrink-0"
        disabled={loading || busy || !value} aria-busy={busy} onClick={() => void add()}>添加应用</Button>
    </div>
    {error && <p role="alert" className="mb-5 text-sm text-danger">{error}，可点击顶部刷新后重试。</p>}
    {loading ? <p className="py-16 text-sm text-text-600">正在读取文件设置…</p> : value && <div className="settings-routing settings-file-open">
      <h2>文件类型</h2>
      <div className="settings-group">
        {([['default_open', '默认方式'], ['json_open', 'JSON 文件'], ['html_open', 'HTML 文件']] as const).map(([key, label]) => <div className="settings-row" key={key}>
          <h3>{label}</h3>
          <Select aria-label={label} className="settings-value" value={value[key]} options={key === 'default_open' ? options : [inherit, ...options]}
            disabled={busy} onValueChange={choice => void change(key, choice)} />
        </div>)}
      </div>
      <h2>自定义应用</h2>
      <div className="settings-group">
        {value.custom_apps.map(app => <div key={app} className="settings-row"><div className="settings-app-label"><FileApplicationIcon application={applicationName(app)} /><h3>{applicationName(app)}</h3></div>
          <IconButton label={`删除 ${applicationName(app)}`} title="删除" className="text-danger ui-danger" disabled={busy} onClick={() => void remove(app)}><Trash2 size={16} aria-hidden="true" /></IconButton>
        </div>)}
      </div>
    </div>}
  </>;
}
