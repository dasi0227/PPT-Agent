import { useMemo, useState } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import type { AssetDetail } from '../../api/assets';
import { RepositoryPreviewCache } from './RepositoryPreviewCache';
import { ComponentPreview } from './ComponentPreview';
import { ThemePreview } from './ThemePreview';
import { SegmentedControl } from './RepositoryPrimitives';
import { themeShowcaseModes, type ThemeShowcaseMode } from './themeShowcase';

export function AssetPreview({ item }: { item: AssetDetail }) {
  const [mode, setMode] = useState<ThemeShowcaseMode>('cover');
  const theme = useMemo(() => item.type === 'theme' ? ({
    id: item.id, name: item.name, content_state: 'ready' as const,
    appearance: { hash: String(item.updated_at), theme_css_url: 'data:text/css;charset=utf-8,' + encodeURIComponent(item.content), decoration_tokens: {} },
  }) : null, [item.id, item.name, item.type, item.updated_at, item.content]);
  if (theme) return <>
    <div className="mb-5 flex shrink-0 justify-end"><SegmentedControl value={mode} options={themeShowcaseModes} onChange={setMode} label="预览页面" /></div>
    <div className="flex min-h-0 flex-1 items-center justify-center"><div className="aspect-video max-h-full w-full max-w-5xl overflow-hidden rounded-lg border border-border-strong">
      <RepositoryPreviewCache activeKey={mode} entries={themeShowcaseModes.map(({ value }) => ({
        key: value, content: <ThemePreview theme={theme} mode={value} />,
      }))} />
    </div></div>
  </>;
  if (item.type === 'component') return <div className="aspect-video max-h-full w-full max-w-5xl overflow-hidden rounded-lg border border-border-strong bg-white">
    <ComponentPreview title={item.name + ' 组件预览'} html={item.content} />
  </div>;
  if (item.type === 'skill') return <article className="prose prose-sm mx-auto max-w-4xl rounded-lg border border-border bg-surface px-7 py-8 text-text-900 shadow-card prose-headings:font-bold prose-headings:tracking-[-0.015em] prose-headings:text-text-900 prose-h1:mb-3 prose-h1:text-2xl prose-h2:mb-3 prose-h2:mt-8 prose-h2:text-base prose-p:text-[13px] prose-p:leading-7 prose-p:text-text-700 prose-li:text-[13px] prose-li:leading-6 prose-blockquote:border-accent prose-blockquote:bg-accent-soft/60 prose-blockquote:px-4 prose-blockquote:py-1 prose-blockquote:not-italic prose-code:rounded prose-code:bg-panel-muted prose-code:px-1 prose-code:py-0.5 prose-code:text-[12px] prose-pre:border prose-pre:border-border prose-pre:bg-panel-muted prose-pre:text-text-800 [&_pre_code]:rounded-none [&_pre_code]:bg-transparent [&_pre_code]:p-0 [&_pre_code]:text-inherit">
    <ReactMarkdown remarkPlugins={[remarkGfm]}>{item.content}</ReactMarkdown>
  </article>;
  return <article className="w-fit max-w-full rounded-xl border border-border bg-surface px-8 py-8 shadow-card md:px-11 md:py-10">
    <p className="m-0 whitespace-pre-wrap break-words text-lg font-semibold leading-8 tracking-[0.01em] text-text-800 md:text-xl md:leading-9">{item.content}</p>
  </article>;
}
