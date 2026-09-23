import { memo, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { PreviewLoading } from '../../components/ui/PreviewLoading';
import { cn } from '../../lib/utils';

const WIDTH = 960;
const HEIGHT = 540;

// A neutral workbench, not a presentation. Component HTML owns its visuals;
// no theme tokens, public slide roles, decorations or theme API dependencies.
function componentDocument(html: string) {
  return `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8">
<link rel="stylesheet" href="/api/v1/runtime/fonts.css">
<style>
html,body{margin:0;width:100%;height:100%;overflow:hidden}
body{display:grid;place-items:center;box-sizing:border-box;padding:40px;background:#fff;color:#263241;font:18px/1.5 "Noto Sans SC",sans-serif}
.component-preview-stage{width:100%;max-width:880px;display:grid;place-items:center}
.component-preview-stage>svg{width:100%;height:auto}
</style></head><body><div class="component-preview-stage">${html}</div></body></html>`;
}

export const ComponentPreview = memo(function ComponentPreview({ html, title, miniature = false, className }: {
  html: string; title: string; miniature?: boolean; className?: string;
}) {
  const host = useRef<HTMLDivElement>(null);
  const [visible, setVisible] = useState(!miniature);
  const [scale, setScale] = useState(0);
  const [loaded, setLoaded] = useState(false);
  const [failed, setFailed] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const document = useMemo(() => componentDocument(html), [html]);

  useEffect(() => {
    if (!miniature || !host.current) return;
    if (typeof IntersectionObserver === 'undefined') { setVisible(true); return; }
    const observer = new IntersectionObserver(entries => {
      if (entries.some(entry => entry.isIntersecting)) { setVisible(true); observer.disconnect(); }
    });
    observer.observe(host.current);
    return () => observer.disconnect();
  }, [miniature]);

  useLayoutEffect(() => {
    const container = host.current;
    if (!container) return;
    const measure = () => {
      const { width, height } = container.getBoundingClientRect();
      if (width && height) setScale(Math.min(width / WIDTH, height / HEIGHT));
    };
    measure();
    if (typeof ResizeObserver === 'undefined') {
      window.addEventListener('resize', measure);
      return () => window.removeEventListener('resize', measure);
    }
    const observer = new ResizeObserver(measure);
    observer.observe(container);
    return () => observer.disconnect();
  }, []);

  useEffect(() => {
    setLoaded(false);
    setFailed(false);
  }, [document, attempt]);

  useEffect(() => {
    if (!visible || loaded || failed) return;
    const timer = window.setTimeout(() => setFailed(true), 30000);
    return () => window.clearTimeout(timer);
  }, [visible, loaded, failed, document, attempt]);

  return (
    <div ref={host} className={cn('relative h-full w-full overflow-hidden bg-white', miniature && 'pointer-events-none', className)}>
      {visible && <iframe
        key={attempt}
        title={title}
        sandbox="allow-scripts"
        srcDoc={document}
        aria-hidden={!loaded || miniature || undefined}
        tabIndex={miniature || !loaded ? -1 : undefined}
        className="absolute left-1/2 top-1/2 border-0"
        style={{ width: WIDTH, height: HEIGHT, marginLeft: -WIDTH / 2, marginTop: -HEIGHT / 2, transform: `scale(${scale})`, transformOrigin: 'center', opacity: loaded ? 1 : 0, pointerEvents: loaded ? undefined : 'none' }}
        onLoad={() => { setLoaded(true); setFailed(false); }}
        onError={() => setFailed(true)}
      />}
      {visible && !loaded && !failed && <PreviewLoading miniature={miniature} />}
      {failed && !loaded && !miniature && <div role="alert" className="absolute inset-0 flex items-center justify-center gap-3 text-sm text-text-600">
        <span>组件预览加载失败</span>
        <button type="button" className="text-accent" onClick={() => setAttempt(value => value + 1)}>重试</button>
      </div>}
    </div>
  );
});
