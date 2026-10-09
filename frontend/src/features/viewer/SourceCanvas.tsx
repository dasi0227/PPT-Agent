import type { ReactNode } from 'react';
import { Copy } from 'lucide-react';
import { Button } from '../../components/ui/primitives';
import { showGlobalError, showGlobalSuccess } from '../../stores/toastStore';
import './source.css';

export function SourceCanvas({ title, language, label, copyText, notice, children }: {
  title: string; language: 'HTML' | 'JSON'; label: string; copyText?: string;
  notice?: ReactNode; children: ReactNode;
}) {
  return <section className="source-canvas flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden bg-surface" aria-label={label}>
    <header className="flex shrink-0 items-center justify-between gap-3 border-b border-border px-4 py-3 text-sm">
      <div className="flex min-w-0 items-baseline gap-3">
        <h1 className="truncate font-bold" title={title}>{title}</h1>
        <span className="shrink-0 whitespace-nowrap text-xs text-text-600">只读 {language}</span>
      </div>
      <Button variant="ghost" className="shrink-0 px-2" title={`复制 ${language} 源码`} disabled={copyText === undefined} onClick={async () => {
        if (copyText === undefined) return;
        try { await navigator.clipboard.writeText(copyText); showGlobalSuccess(`已复制 ${language} 源码`); }
        catch { showGlobalError('复制失败，请选中源码后手动复制。'); }
      }}><Copy className="h-3.5 w-3.5" strokeWidth={1.75} aria-hidden="true" />复制</Button>
    </header>
    {notice && <div className="shrink-0 px-4 pt-4">{notice}</div>}
    {children}
  </section>;
}
