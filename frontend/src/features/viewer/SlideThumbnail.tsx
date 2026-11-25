import { useEffect, useRef, useState } from 'react';
import { ImageOff } from 'lucide-react';
import { projectsApi } from '../../api/projects';
import type { ProjectContentSnapshot, Slide } from '../../api/types';
import { Skeleton } from '../../components/ui/primitives';
import { cn } from '../../lib/utils';
import { hasRenderedHTML } from '../deck/selectors';

export function SlideThumbnail({ slide, snapshot, className, cachedOnly = false }: {
  slide: Slide;
  snapshot: ProjectContentSnapshot;
  className?: string;
  cachedOnly?: boolean;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const [visible, setVisible] = useState(false);
  const [result, setResult] = useState<{ version: string; src?: string }>();
  const available = hasRenderedHTML(slide);
  const url = projectsApi.thumbnailUrl(snapshot.project_id, slide.id);
  const version = JSON.stringify([
    url, slide.html_hash, snapshot.hashes.outline, snapshot.hashes.meta, snapshot.project_title,
    snapshot.theme, snapshot.appearance?.hash, snapshot.decorations, snapshot.logo?.hash,
  ]);

  useEffect(() => {
    const node = ref.current;
    if (!node || visible) return;
    if (typeof IntersectionObserver === 'undefined') {
      setVisible(true);
      return;
    }
    const observer = new IntersectionObserver(entries => {
      if (entries.some(entry => entry.isIntersecting)) {
        setVisible(true);
        observer.disconnect();
      }
    }, { rootMargin: '160px' });
    observer.observe(node);
    return () => observer.disconnect();
  }, [visible]);

  useEffect(() => {
    if (!visible || !available) return;
    const controller = new AbortController();
    let src: string | undefined;
    void fetch(url, { signal: controller.signal, mode: 'same-origin', cache: cachedOnly ? 'only-if-cached' : 'no-cache' }).then(async response => {
      if (!response.ok) throw new Error('缩略图读取失败');
      const image = await response.blob();
      if (controller.signal.aborted) return;
      src = URL.createObjectURL(image);
      setResult({ version, src });
    }).catch(() => {
      if (!controller.signal.aborted) setResult({ version });
    });
    return () => {
      controller.abort();
      if (src) URL.revokeObjectURL(src);
    };
  }, [visible, available, url, version, cachedOnly]);

  const current = result?.version === version ? result : undefined;
  return (
    <div ref={ref} className={cn('relative aspect-video w-full min-w-0 overflow-hidden', !cachedOnly && 'bg-surface', className)}
      aria-busy={!cachedOnly && available && !current}>
      {!cachedOnly && (!available ? (
        <div className="flex h-full items-center justify-center text-[10px] font-normal text-text-400">未生成</div>
      ) : current && !current.src ? (
        <div className="flex h-full items-center justify-center text-text-400"><ImageOff className="h-4 w-4" aria-hidden="true" /></div>
      ) : !current && <Skeleton className="absolute inset-0 h-full w-full rounded-none" />)}
      {available && current?.src && <img src={current.src} alt="" decoding="async" draggable={false}
        className="pointer-events-none absolute inset-0 h-full w-full object-contain" />}
    </div>
  );
}
