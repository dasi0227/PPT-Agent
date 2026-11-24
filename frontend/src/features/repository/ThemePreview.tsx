import { memo, useEffect, useMemo, useRef, useState } from 'react';
import { PreviewLoading } from '../../components/ui/PreviewLoading';
import type { Theme } from '../../api/types';
import { IsolatedSlidePreview } from '../viewer/IsolatedSlidePreview';
import type { RuntimeSlide } from '../viewer/previewProtocol';
import chart from './examples/chart.html?raw';
import content from './examples/content.html?raw';
import cover from './examples/cover.html?raw';
import type { ThemeShowcaseMode } from './themeShowcase';

const examples: Record<ThemeShowcaseMode, string> = { cover, content, chart };

// Same HTML, scaling and iframe execution path as the project canvas.
export const ThemePreview = memo(function ThemePreview({ theme, mode = 'cover', miniature = false }: { theme: Pick<Theme, 'id' | 'name' | 'appearance' | 'content_state'>; mode?: ThemeShowcaseMode; miniature?: boolean }) {
  const host = useRef<HTMLDivElement>(null);
  const [visible, setVisible] = useState(!miniature);
  useEffect(() => {
    if (!miniature || !host.current) return;
    if (typeof IntersectionObserver === 'undefined') { setVisible(true); return; }
    const observer = new IntersectionObserver(entries => { if (entries.some(entry => entry.isIntersecting)) { setVisible(true); observer.disconnect(); } });
    observer.observe(host.current);
    return () => observer.disconnect();
  }, [miniature]);
  const slides = useMemo<RuntimeSlide[]>(() => theme.content_state === 'ready' && theme.appearance ? [{
    id: `example-${mode}`, html: examples[mode],
    frame: {
      slide_id: `example-${mode}`, theme_id: theme.id, appearance: theme.appearance,
      canvas: { width: 1920, height: 1080, aspect_ratio: '16:9' }, logo: null, deck_title: '主题契约研究',
      ordinal: mode === 'cover' ? 1 : mode === 'content' ? 2 : 3, total: 3, purpose: mode === 'cover' ? 'cover' : 'content_explanation',
      section: { id: 'examples', title: '内容与表达', index: 1 },
      decorations: { slide_pagination: 'bottom-right', deck_title: 'none', section_title: 'none', logo: 'none' },
    },
  }] : [], [mode, theme]);
  return <div ref={host} className={`relative h-full w-full overflow-hidden ${miniature ? 'pointer-events-none' : ''}`} aria-hidden={miniature || undefined}>
    {visible && slides.length > 0 && <IsolatedSlidePreview passive={miniature} slides={slides} index={0} title={`${theme.name} 主题预览`} className="h-full w-full border-0" />}
    {visible && slides.length === 0 && <PreviewLoading miniature={miniature} />}
  </div>;
});
