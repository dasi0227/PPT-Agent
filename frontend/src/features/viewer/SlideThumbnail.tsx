import { useEffect, useRef, useState } from 'react';
import { ImageOff } from 'lucide-react';
import type { ProjectContentSnapshot, Slide } from '../../api/types';
import { Skeleton } from '../../components/ui/primitives';
import { cn } from '../../lib/utils';
import { hasRenderedHTML } from './useSlideRenderCache';

export function SlideThumbnail({ slide, snapshot, className, cachedOnly = false }: {
  slide: Slide;
  snapshot: ProjectContentSnapshot;
  className?: string;
  cachedOnly?: boolean;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const [visible, setVisible] = useState(false);
  const [loadedURL, setLoadedURL] = useState('');
  const [failedURL, setFailedURL] = useState('');
  const version = snapshot.hashes[`thumbnail:${slide.id}`];
  const url = version
    ? `/api/v1/slides/${encodeURIComponent(slide.id)}/thumbnail?v=${encodeURIComponent(version)}${cachedOnly ? '&cached=1' : ''}`
    : '';

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

  const ready = Boolean(url) && loadedURL === url;
  const failed = Boolean(url) && failedURL === url;
  return (
    <div ref={ref} className={cn('relative aspect-video w-full min-w-0 overflow-hidden', !cachedOnly && 'bg-surface', className)} aria-busy={!cachedOnly && hasRenderedHTML(slide) && !ready && !failed}>
      {!cachedOnly && (!hasRenderedHTML(slide) ? (
        <div className="flex h-full items-center justify-center text-[10px] font-normal text-text-400">未生成</div>
      ) : failed ? (
        <div className="flex h-full items-center justify-center text-text-400"><ImageOff className="h-4 w-4" aria-hidden="true" /></div>
      ) : !ready && <Skeleton className="absolute inset-0 h-full w-full rounded-none" />)}
      {visible && url && (
        <img
          key={url}
          src={url}
          alt=""
          loading="lazy"
          decoding="async"
          draggable={false}
          className={cn('pointer-events-none absolute inset-0 h-full w-full object-contain', !ready && 'invisible')}
          onLoad={() => { setLoadedURL(url); setFailedURL(''); }}
          onError={() => setFailedURL(url)}
        />
      )}
    </div>
  );
}
