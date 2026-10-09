import { useEffect, useId, useState } from 'react';
import { repositoriesApi } from '../../api/repositories';
import { ComponentPreview } from '../repository/ComponentPreview';
import { LongContent } from './LongContent';
import { MarkdownMessage } from './MarkdownMessage';
import { TimelineChevron, TimelineDisclosure } from './TimelineDisclosure';

interface Resource {
  kind: 'skill' | 'component';
  id: string;
  name: string;
}

interface ResourceContent {
  identity: string;
  description: string;
  content?: string;
  error?: string;
}

export function ResourceActivityCard({ resource }: { resource: Resource }) {
  const { kind, id, name } = resource;
  const identity = `${kind}:${id}`;
  const contentId = useId();
  const [open, setOpen] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const [result, setResult] = useState<ResourceContent>();

  useEffect(() => {
    let canceled = false;
    setResult(undefined);
    const load = async (): Promise<ResourceContent> => {
      if (kind === 'skill') {
        const document = await repositoriesApi.getSkill(id);
        return {
          identity,
          description: document.description,
          content: document.content_state === 'ready' ? document.content : undefined,
          error: document.content_state !== 'ready' || document.content === undefined
            ? document.content_error || '技能内容不可用。' : undefined,
        };
      }
      const document = await repositoriesApi.getComponent(id);
      return {
        identity,
        description: document.description,
        content: document.content_state === 'ready' ? document.html : undefined,
        error: document.content_state !== 'ready' || document.html === undefined
          ? document.content_error || '组件内容不可用。' : undefined,
      };
    };
    void load().then(value => {
      if (!canceled) setResult(value);
    }).catch(cause => {
      if (!canceled) setResult({ identity, description: '', error: cause instanceof Error ? cause.message : '资源加载失败。' });
    });
    return () => { canceled = true; };
  }, [identity, kind, id, attempt]);

  const current = result?.identity === identity ? result : undefined;
  // Skill metadata belongs to the card header; render only its Markdown body.
  const markdown = kind === 'skill' ? current?.content?.replace(/^\uFEFF?---\r?\n[\s\S]*?\r?\n---(?:\r?\n|$)/, '') : undefined;

  return <section className="min-w-0 overflow-hidden rounded-[10px] border border-border bg-timeline-card">
    <button type="button" aria-expanded={open} aria-controls={contentId} onClick={() => setOpen(value => !value)}
      className="timeline-disclosure-trigger ui-interactive grid w-full min-w-0 grid-cols-[minmax(0,1fr)_14px] items-center gap-2.5 p-3 text-left">
      <span className="min-w-0">
        <span className="block truncate text-xs font-semibold leading-5 text-text-800" title={name}>{name}</span>
        {current?.description && <span className="mt-1 block truncate text-[11px] leading-[18px] text-text-600" title={current.description}>{current.description}</span>}
      </span>
      <TimelineChevron open={open} />
    </button>
    <TimelineDisclosure id={contentId} open={open}>
      {open && <div className="border-t border-border">
        {current?.error ? <div role="status" className="px-4 py-3 text-xs text-text-600">
          {current.error}<button type="button" onClick={() => setAttempt(value => value + 1)} className="ui-interactive ml-2 rounded px-2 py-1">重试</button>
        </div> : current?.content === undefined ? <p role="status" className="px-4 py-3 text-xs text-text-600">正在加载…</p>
          : kind === 'skill' ? <LongContent maxHeight={320} contentClassName="px-4 py-3.5" controlsClassName="mt-0 pb-3"
            fadeClassName="from-timeline-card/0 via-timeline-card/90 to-timeline-card" buttonClassName="bg-timeline-card text-[11px] shadow-none">
            <MarkdownMessage content={markdown ?? ''} className="text-xs leading-[1.8] [overflow-wrap:anywhere] prose-headings:text-[13px] prose-headings:leading-5 [&_>:first-child]:mt-0 [&_>:last-child]:mb-0" />
          </LongContent> : <div className="aspect-video w-full">
            <ComponentPreview html={current.content} title={`${name} 组件预览`} />
          </div>}
      </div>}
    </TimelineDisclosure>
  </section>;
}
