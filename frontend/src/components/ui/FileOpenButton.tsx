import { useRef, useState, type ReactNode } from 'react';
import { fileActionLabel, filesApi } from '../../api/files';
import { useFileSettingsStore } from '../../stores/fileSettingsStore';
import { showGlobalError } from '../../stores/toastStore';

export function FileOpenButton({ url, children, className, label }: {
  url: string; children: ReactNode; className?: string; label?: string;
}) {
  const value = useFileSettingsStore(state => state.value);
  const [busy, setBusy] = useState(false);
  const inFlight = useRef(false);
  const action = fileActionLabel(value);
  return <button type="button" className={className} title={action} aria-label={label ? `${label}：${action}` : action}
    disabled={busy} aria-busy={busy} onClick={async event => {
      event.stopPropagation();
      if (inFlight.current) return;
      inFlight.current = true; setBusy(true);
      try { await filesApi.open(url); }
      catch (cause) { showGlobalError(cause instanceof Error ? cause.message : '文件打开失败'); }
      finally { inFlight.current = false; setBusy(false); }
    }}>{children}</button>;
}
