import { useEffect, useState } from 'react';
import { APIError } from '../../api/client';
import { readHTMLSource, type HTMLSourceDocument } from '../../api/htmlSource';
import { HTMLSource } from '../../components/HTMLSource';
import { Button, InlineNotice } from '../../components/ui/primitives';
import { showGlobalError, showGlobalSuccess } from '../../stores/toastStore';
import { useProjectStore } from '../../stores/projectStore';
import { formatHTMLForDisplay } from './htmlSourceFormatClient';

export function HTMLSourceView({ projectId, slideId, title, ordinal, hash, sceneRevision, available }: {
  projectId: string; slideId?: string; title?: string; ordinal: number;
  hash?: string; sceneRevision?: number; available: boolean;
}) {
  const identity = `${projectId}:${slideId}:${hash}:${sceneRevision}`;
  const [result, setResult] = useState<{ identity: string; document?: HTMLSourceDocument; error?: string }>();
  const [formatted, setFormatted] = useState<{ identity: string; content: string }>();
  const [retry, setRetry] = useState(0);
  useEffect(() => {
    if (!slideId || !available) return;
    let canceled = false;
    setResult(undefined);
    readHTMLSource(projectId, slideId).then(document => {
      if (document.slide_id !== slideId || document.project_id !== projectId || (hash && document.source_hash !== hash)
        || (sceneRevision !== undefined && document.scene_revision !== sceneRevision)) throw new Error('页面内容已更新，请刷新后重试。');
      if (!canceled) setResult({ identity, document });
    }).catch(error => {
      if (!canceled) setResult({ identity, error: error instanceof APIError && error.code === 'SOURCE_NOT_FOUND' ? '此页 HTML 尚未生成。' : error instanceof Error ? error.message : '源码加载失败，请重试。' });
    });
    return () => { canceled = true; };
  }, [identity, projectId, slideId, hash, sceneRevision, available, retry]);
  const current = result?.identity === identity ? result : undefined;
  useEffect(() => {
    if (!current?.document) return;
    let canceled = false;
    void formatHTMLForDisplay(current.document.content, current.document.source_hash).then((content) => {
      if (!canceled) setFormatted({ identity, content });
    });
    return () => { canceled = true; };
  }, [current?.document, identity]);
  const displayContent = formatted?.identity === identity ? formatted.content : undefined;
  return <section className="flex min-h-0 flex-1 flex-col bg-surface" aria-label="幻灯片源码">
    <header className="flex flex-wrap items-center justify-between gap-3 border-b border-border px-4 py-3 text-sm">
      <div className="min-w-0"><strong>{slideId ? `第 ${ordinal} 页 · ${title ?? ''}` : '请选择页面'}</strong><span className="ml-3 text-xs text-text-600">只读 HTML</span></div>
      <Button variant="secondary" disabled={!current?.document} onClick={() => {
        if (!current?.document) return;
        void navigator.clipboard.writeText(current.document.content).then(() => showGlobalSuccess('已复制 HTML 源码')).catch(() => showGlobalError('复制失败，请选中源码后手动复制。'));
      }}>复制源码</Button>
    </header>
    {!slideId || !available ? <p className="m-auto p-6 text-sm text-text-600">{slideId ? '此页 HTML 尚未生成。' : '选择具体页面后可查看 HTML 源码。'}</p>
      : current?.error ? <InlineNotice tone="danger" className="m-4 flex items-center justify-between gap-3"><span>{current.error}</span><Button variant="secondary" onClick={async () => {
        await useProjectStore.getState().loadProjectContent(projectId);
        setRetry(value => value + 1);
      }}>重试</Button></InlineNotice>
        : current?.document ? displayContent === undefined
          ? <p role="status" className="m-auto p-6 text-sm text-text-600">正在整理源码…</p>
          : <HTMLSource text={displayContent} />
          : <p role="status" className="m-auto p-6 text-sm text-text-600">正在加载源码…</p>}
  </section>;
}
