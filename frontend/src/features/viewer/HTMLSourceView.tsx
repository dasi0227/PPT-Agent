import { useEffect, useState } from 'react';
import { HTMLSource } from '../../components/HTMLSource';
import { formatHTMLForDisplay } from './htmlSourceFormatClient';
import { SourceCanvas } from './SourceCanvas';

export function HTMLSourceView({ slideId, title, ordinal, hash, html }: {
  slideId?: string; title?: string; ordinal: number;
  hash?: string; html?: string | null;
}) {
  const [formatted, setFormatted] = useState<{ hash: string; text: string }>();
  useEffect(() => {
    if (html == null || !hash) return;
    let canceled = false;
    void formatHTMLForDisplay(html, hash).then(text => {
      if (!canceled) setFormatted({ hash, text });
    }).catch(() => { /* Keep showing the original source if formatting fails. */ });
    return () => { canceled = true; };
  }, [html, hash]);

  return <SourceCanvas title={slideId ? `第 ${ordinal} 页 · ${title ?? ''}` : '请选择页面'} language="HTML" label="幻灯片源码" copyText={html ?? undefined}>
    {html == null ? <p className="m-auto p-6 text-sm text-text-600">{slideId ? '此页 HTML 尚未生成。' : '选择具体页面后可查看 HTML 源码。'}</p>
      : <HTMLSource text={formatted && formatted.hash === hash ? formatted.text : html} />}
  </SourceCanvas>;
}
