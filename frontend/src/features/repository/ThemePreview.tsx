import { memo, useEffect, useMemo, useRef, useState, useSyncExternalStore } from 'react';
import { PreviewLoading } from '../../components/ui/PreviewLoading';
import { repositoriesApi } from '../../api/repositories';
import type { Theme } from '../../api/types';
import { IsolatedSlidePreview } from '../viewer/IsolatedSlidePreview';
import type { RuntimeSlide } from '../viewer/previewProtocol';
import { themeShowcaseModes, type ThemeShowcaseMode } from './themeShowcase';
import { RepositoryPreviewCache } from './RepositoryPreviewCache';

const examples = new Map<string, Promise<string>>();
let exampleVersion = 0;
const exampleListeners = new Set<() => void>();
const subscribeExamples = (listener: () => void) => {
  exampleListeners.add(listener);
  return () => { exampleListeners.delete(listener); };
};
export function clearThemeExampleCache() {
  examples.clear();
  exampleVersion++;
  exampleListeners.forEach(listener => listener());
}
function loadExample(mode: ThemeShowcaseMode) {
  let pending = examples.get(mode);
  if (!pending) {
    pending = repositoriesApi.themeExample(mode).then(value => value.html).catch(error => {
      if (examples.get(mode) === pending) examples.delete(mode);
      throw error;
    });
    examples.set(mode, pending);
  }
  return pending;
}

// Same HTML, scaling and iframe execution path as the project canvas.
export const ThemePreview = memo(function ThemePreview({ theme, mode = 'cover', miniature = false }: { theme: Theme; mode?: ThemeShowcaseMode; miniature?: boolean }) {
  const version = useSyncExternalStore(subscribeExamples, () => exampleVersion);
  const host = useRef<HTMLDivElement>(null);
  const [visible, setVisible] = useState(!miniature);
  const [html, setHTML] = useState('');
  const [error, setError] = useState('');
  const [retry, setRetry] = useState(0);
  useEffect(() => {
    if (!miniature || !host.current) return;
    if (typeof IntersectionObserver === 'undefined') { setVisible(true); return; }
    const observer = new IntersectionObserver(entries => { if (entries.some(entry => entry.isIntersecting)) { setVisible(true); observer.disconnect(); } });
    observer.observe(host.current);
    return () => observer.disconnect();
  }, [miniature]);
  useEffect(() => {
    if (!visible) return;
    let active = true; setError('');
    void loadExample(mode).then(value => { if (active) setHTML(value); }).catch(cause => { if (active) setError(cause instanceof Error ? cause.message : '示例加载失败'); });
    return () => { active = false; };
  }, [mode, visible, retry, version]);
  const slides = useMemo<RuntimeSlide[]>(() => html ? [{
    id: `example-${mode}`, html,
    frame: {
      slide_id: `example-${mode}`, theme_id: theme.id, appearance: theme.appearance,
      canvas: { width: 1920, height: 1080, aspect_ratio: '16:9' }, key_message: '', deck_title: '主题契约研究',
      ordinal: mode === 'cover' ? 1 : mode === 'content' ? 2 : 3, total: 3, role: mode === 'cover' ? 'cover' : 'content',
      section: { id: 'examples', title: '内容与表达', index: 1 },
      decorations: { page_number: 'bottom-right', deck_title: 'none', section_title: 'none', key_message: 'none' },
    },
  }] : [], [html, mode, theme]);
  return <div ref={host} className={`relative h-full w-full overflow-hidden ${miniature ? 'pointer-events-none' : ''}`} aria-hidden={miniature || undefined}>
    {visible && html && <IsolatedSlidePreview passive={miniature} slides={slides} index={0} title={`${theme.name} 主题预览`} className="h-full w-full border-0" />}
    {visible && !html && !error && <PreviewLoading miniature={miniature} />}
    {error && !miniature && <div role="alert" className="absolute inset-0 flex items-center justify-center gap-3 text-sm text-danger"><span>{error}</span><button type="button" onClick={() => setRetry(value => value + 1)}>重试</button></div>}
  </div>;
});

function previewKey(theme: Theme, mode: ThemeShowcaseMode) {
  return JSON.stringify([theme.id, theme.appearance?.hash ?? theme.style_hash, mode]);
}

export function ThemePreviewGallery({ themes, selected, mode }: { themes: Theme[]; selected: Theme; mode: ThemeShowcaseMode }) {
  const entries = useMemo(() => themes.flatMap(theme => themeShowcaseModes.map(example => ({
    key: previewKey(theme, example.value),
    content: <ThemePreview theme={theme} mode={example.value} />,
  }))), [themes]);
  return <RepositoryPreviewCache entries={entries} activeKey={previewKey(selected, mode)} />;
}
