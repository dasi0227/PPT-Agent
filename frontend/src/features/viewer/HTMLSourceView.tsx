import { useEffect, useState } from 'react';
import { APIError } from '../../api/client';
import { readHTMLSource, type HTMLSourceDocument } from '../../api/htmlSource';
import { HTMLSource } from '../../components/HTMLSource';
import { Button, InlineNotice } from '../../components/ui/primitives';
import { useProjectStore } from '../../stores/projectStore';
import { formatHTMLForDisplay } from './htmlSourceFormatClient';
import { SourceCanvas } from './SourceCanvas';

export function HTMLSourceView({ projectId, slideId, title, ordinal, hash, sceneRevision, available }: {
  projectId: string; slideId?: string; title?: string; ordinal: number;
  hash?: string; sceneRevision?: number; available: boolean;
}) {
  const identity = `${projectId}:${slideId}:${hash}:${sceneRevision}`;
  const [result, setResult] = useState<{ identity: string; document?: HTMLSourceDocument; display?: string; error?: string }>();
  const [retry, setRetry] = useState(0);
  useEffect(() => {
    if (!slideId || !available) return;
    let canceled = false;
    setResult(undefined);
    readHTMLSource(projectId, slideId).then(async document => {
      if (document.slide_id !== slideId || document.project_id !== projectId || (hash && document.source_hash !== hash)
        || (sceneRevision !== undefined && document.scene_revision !== sceneRevision)) throw new Error('页面内容已更新，请刷新后重试。');
      if (!canceled) setResult({ identity, document });
      const display = await formatHTMLForDisplay(document.content, document.source_hash);
      if (!canceled) setResult({ identity, document, display });
    }).catch(error => {
      if (!canceled) setResult({ identity, error: error instanceof APIError && error.code === 'SOURCE_NOT_FOUND' ? '此页 HTML 尚未生成。' : error instanceof Error ? error.message : '源码加载失败，请重试。' });
    });
    return () => { canceled = true; };
  }, [identity, projectId, slideId, hash, sceneRevision, available, retry]);
  const current = result?.identity === identity ? result : undefined;
  return <SourceCanvas title={slideId ? `第 ${ordinal} 页 · ${title ?? ''}` : '请选择页面'} language="HTML" label="幻灯片源码" copyText={current?.document?.content}>
    {!slideId || !available ? <p className="m-auto p-6 text-sm text-text-600">{slideId ? '此页 HTML 尚未生成。' : '选择具体页面后可查看 HTML 源码。'}</p>
      : current?.error ? <InlineNotice tone="danger" className="m-4 flex items-center justify-between gap-3"><span>{current.error}</span><Button variant="secondary" onClick={async () => {
        await useProjectStore.getState().loadProjectContent(projectId);
        setRetry(value => value + 1);
      }}>重试</Button></InlineNotice>
        : current?.document ? <HTMLSource text={current.display ?? current.document.content} />
          : <p role="status" className="m-auto p-6 text-sm text-text-600">正在加载源码…</p>}
  </SourceCanvas>;
}
