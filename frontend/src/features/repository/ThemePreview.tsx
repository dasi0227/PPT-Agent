import { useEffect, useMemo, useRef, useState } from 'react';
import { repositoriesApi } from '../../api/repositories';
import type { Theme } from '../../api/types';
import { IsolatedSlidePreview } from '../viewer/IsolatedSlidePreview';
import type { RuntimeSlide } from '../viewer/previewProtocol';
import type { ThemeShowcaseMode } from './themeShowcase';

const examples = new Map<string, Promise<string>>();
function loadExample(mode: ThemeShowcaseMode) {
  let pending = examples.get(mode);
  if (!pending) {
    pending = repositoriesApi.themeExample(mode).then(value => value.html).catch(error => { examples.delete(mode); throw error; });
    examples.set(mode, pending);
  }
  return pending;
}

// Same HTML, scaling and iframe execution path as the project canvas.
export function ThemePreview({ theme, mode = 'cover', miniature = false }: { theme: Theme; mode?: ThemeShowcaseMode; miniature?: boolean }) {
  const host = useRef<HTMLDivElement>(null);
  const [visible, setVisible] = useState(!miniature);
  const [html, setHTML] = useState('');
  const [error, setError] = useState('');
  const [retry, setRetry] = useState(0);
  useEffect(() => {
    if (!miniature || !host.current) return;
    const observer = new IntersectionObserver(entries => { if (entries.some(entry => entry.isIntersecting)) { setVisible(true); observer.disconnect(); } });
    observer.observe(host.current);
    return () => observer.disconnect();
  }, [miniature]);
  useEffect(() => {
    if (!visible) return;
    let active = true; setError(''); setHTML('');
    void loadExample(mode).then(value => { if (active) setHTML(value); }).catch(cause => { if (active) setError(cause instanceof Error ? cause.message : '示例加载失败'); });
    return () => { active = false; };
  }, [mode, visible, retry]);
  const slides = useMemo<RuntimeSlide[]>(() => html ? [{
    id: `example-${mode}`, html,
    frame: {
      slide_id: `example-${mode}`, theme_id: theme.id, appearance: theme.appearance,
      canvas: { width: 1920, height: 1080, aspect_ratio: '16:9' }, deck_title: '主题契约研究',
      ordinal: mode === 'cover' ? 1 : mode === 'content' ? 2 : 3, total: 3, role: mode === 'cover' ? 'cover' : 'content',
      section: { id: 'examples', title: '内容与表达', index: 1 }, numbering: { visible: mode !== 'cover', format: 'number' },
      chrome: [{ type: 'page_number', placement: 'bottom-right', style: 'mono compact' }],
    },
  }] : [], [html, mode, theme]);
  return <div ref={host} className={`relative h-full w-full overflow-hidden ${miniature ? 'pointer-events-none' : ''}`} aria-hidden={miniature || undefined}>
    {visible && html && <IsolatedSlidePreview passive={miniature} slides={slides} index={0} title={`${theme.name} 主题预览`} className="h-full w-full border-0" />}
    {error && !miniature && <div className="absolute inset-0 flex items-center justify-center gap-3 text-sm text-danger"><span>{error}</span><button onClick={() => setRetry(value => value + 1)}>重试</button></div>}
  </div>;
}
