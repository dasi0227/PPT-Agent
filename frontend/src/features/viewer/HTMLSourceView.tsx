import { useEffect, useState } from 'react';
import { HTMLSource } from '../../components/HTMLSource';
import { Button } from '../../components/ui/primitives';
import { FormModal } from '../../components/ui/modal-form';
import { useProjectStore } from '../../stores/projectStore';
import { showGlobalSuccess } from '../../stores/toastStore';
import { formatHTMLForDisplay } from './htmlSourceFormatClient';
import { SourceCanvas } from './SourceCanvas';

export function HTMLSourceView({ projectId, slideId, title, ordinal, hash, html, blocked }: {
  projectId: string; slideId: string; title: string; ordinal: number;
  hash?: string; html?: string | null; blocked?: string;
}) {
  const [formatted, setFormatted] = useState<{ hash: string; text: string }>();
  const [draft, setDraft] = useState<{ html: string; hash: string | null }>();
  useEffect(() => {
    if (html == null || !hash) return;
    let canceled = false;
    void formatHTMLForDisplay(html, hash).then(text => {
      if (!canceled) setFormatted({ hash, text });
    }).catch(() => { /* Keep showing the original source if formatting fails. */ });
    return () => { canceled = true; };
  }, [html, hash]);

  return <SourceCanvas title={`第 ${ordinal} 页 · ${title}`} language="HTML" label="幻灯片源码" copyText={html ?? undefined}
    action={<Button variant="ghost" className="px-2" disabled={Boolean(blocked)} title={blocked}
      onClick={() => setDraft({ html: html ?? '', hash: hash ?? null })}>编辑</Button>}>
    {html == null ? <p className="m-auto p-6 text-sm text-text-600">此页 HTML 尚未生成。</p>
      : <HTMLSource text={formatted && formatted.hash === hash ? formatted.text : html} />}
    {draft && <FormModal open title="编辑 HTML" confirmLabel="保存" initialValue={draft.html}
      onOpenChange={open => { if (!open) setDraft(undefined); }}
      validate={() => null}
      renderField={(value, setValue) => <textarea autoFocus aria-label="HTML 源码" spellCheck={false}
        className="h-72 w-full rounded-control border border-border bg-panel p-3 font-mono text-sm text-text-900 outline-none"
        value={value} onChange={event => setValue(event.target.value)} />}
      onSubmit={async content => {
        if (blocked) throw new Error(blocked);
        const store = useProjectStore.getState();
        try {
          await store.saveProjectFile(projectId, draft.hash, { part: 'html', slideId, content });
          showGlobalSuccess('已保存');
        } catch (error) {
          void store.loadProjectContent(projectId);
          throw error;
        }
      }} />}
  </SourceCanvas>;
}
