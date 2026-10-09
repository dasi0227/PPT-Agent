import { useCallback, useEffect, useState, type ReactNode } from 'react';
import { CornerUpLeft } from 'lucide-react';
import type { PublicTarget } from '../../api/types';
import { readHTMLSource } from '../../api/htmlSource';
import { useProjectStore } from '../../stores/projectStore';
import { useDeckStore } from '../../stores/deckStore';
import { orderedSlides } from '../deck/selectors';
import { formatHTMLForDisplay } from '../viewer/htmlSourceFormatClient';
import { LongContent } from './LongContent';
import { TimelineCardHeader } from './TimelineCardHeader';
import { IconButton } from '../../components/ui/primitives';

const openIcon = <CornerUpLeft className="h-3.5 w-3.5" strokeWidth={1.75} aria-hidden="true" />;

export function openSourceTarget(target: PublicTarget) {
  const deck = useDeckStore.getState();
  deck.exitOverview();
  if (target.type === 'deck' && (target.part === 'manifest' || target.part === 'design' || target.part === 'outline')) {
    deck.setActiveDocument(target.part);
  } else {
    deck.setCurrentSlideId(target.type === 'slide' ? target.slide_id ?? null : null);
    deck.setGlobalView(target.part === 'html' ? 'html' : 'outline');
  }
}

function SourceCard({ language, filename, content, error, retry, action }: {
  language: string; filename: string; content?: string; error?: string; retry?: () => void; action: ReactNode;
}) {
  return <section className="min-w-0 overflow-hidden rounded-[10px] border border-border-strong bg-timeline-card" aria-label={`${language} 源码`}>
    <TimelineCardHeader action={action}>
      <span className="min-w-0 flex-1 whitespace-normal pt-1.5 font-mono font-normal text-[11px] text-text-500 [overflow-wrap:anywhere]">{filename}</span>
    </TimelineCardHeader>
    {error ? <div className="px-3.5 pb-4 pt-2 text-xs text-text-600" role="status">
      {error}{retry && <button type="button" onClick={retry} className="ui-interactive ml-2 rounded px-2 py-1">重试</button>}
    </div> : content === undefined ? <p role="status" className="px-3.5 pb-4 pt-2 text-xs text-text-600">正在加载源码…</p>
      : <LongContent horizontalScroll maxHeight={210} contentClassName="w-max min-w-full px-3.5 pt-2 pb-[18px]" controlsClassName="mt-0 pb-3" fadeClassName="from-timeline-card/0 via-timeline-card/90 to-timeline-card" buttonClassName="h-7 bg-timeline-card text-[11px] shadow-none">
        <pre className="m-0 whitespace-pre break-normal font-mono text-xs leading-[1.8] text-text-600 [tab-size:2]"><code>{content || '（空内容）'}</code></pre>
      </LongContent>}
  </section>;
}

function AsyncSourceCard({ identity, load, language, filename, action }: {
  identity: string; load: () => Promise<string>; language: string; filename: string; action: ReactNode;
}) {
  const [attempt, setAttempt] = useState(0);
  const [result, setResult] = useState<{ identity: string; content?: string; error?: string }>();
  useEffect(() => {
    let canceled = false;
    setResult(undefined);
    void load().then(content => {
      if (!canceled) setResult({ identity, content });
    }).catch(cause => {
      if (!canceled) setResult({ identity, error: cause instanceof Error ? cause.message : '源码加载失败，请重试。' });
    });
    return () => { canceled = true; };
  }, [identity, load, attempt]);
  const current = result?.identity === identity ? result : undefined;
  return <SourceCard language={language} filename={filename} content={current?.content} error={current?.error} retry={() => setAttempt(value => value + 1)} action={action} />;
}

export function TargetSourceCard({ target }: { target: PublicTarget }) {
  const projectId = useProjectStore(state => state.activeProjectId);
  const snapshot = useProjectStore(state => projectId ? state.contentByProjectId[projectId] : undefined);
  const slideId = target.slide_id;
  const exists = target.type === 'deck' || orderedSlides(snapshot).some(slide => slide.id === slideId);
  const hash = slideId ? snapshot?.slides_by_id[slideId]?.html_hash : undefined;
  const loadHTML = useCallback(async () => {
    if (!projectId || !slideId || !exists) throw new Error('页面已删除或不可用。');
    const document = await readHTMLSource(projectId, slideId);
    if (document.project_id !== projectId || document.slide_id !== slideId || (hash && document.source_hash !== hash)) {
      throw new Error('页面内容已更新，请稍后重试。');
    }
    return formatHTMLForDisplay(document.content, document.source_hash);
  }, [projectId, slideId, exists, hash]);
  const action = <IconButton label="预览" className="h-7 w-7 rounded" disabled={!projectId || !exists} onClick={() => openSourceTarget(target)}>{openIcon}</IconButton>;
  if (target.part === 'html') return <AsyncSourceCard key={`${projectId}:${slideId}:${hash}`} identity={`${projectId}:${slideId}:${hash}`} load={loadHTML} language="HTML" filename={`${slideId ?? "slide"}.html`} action={action} />;
  const source = target.type === 'slide' ? (slideId ? snapshot?.slides_by_id[slideId]?.spec : undefined)
    : target.part === 'manifest' ? snapshot?.manifest : target.part === 'design' ? snapshot?.design : target.part === 'outline' ? snapshot?.outline : undefined;
  return <SourceCard language="JSON" filename={`.${target.part}.json`} content={source == null ? undefined : JSON.stringify(source, null, 2)} error={!exists ? '页面已删除。' : source == null ? '暂无内容。' : undefined} action={action} />;
}
