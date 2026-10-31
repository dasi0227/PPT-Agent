import { useLayoutEffect, useRef, useState, type CSSProperties } from 'react';
import { cn } from '../../lib/utils';
import './hover-scroll-text.css';

export function HoverScrollText({ children, className }: { children: string; className?: string }) {
  const viewportRef = useRef<HTMLSpanElement>(null);
  const textRef = useRef<HTMLSpanElement>(null);
  const [distance, setDistance] = useState(0);
  const scrollDuration = Math.max(1.5, distance / 32);
  const pauseDuration = 0.5;
  const cycleDuration = scrollDuration + pauseDuration * 2;

  useLayoutEffect(() => {
    const viewport = viewportRef.current;
    const text = textRef.current;
    if (!viewport || !text) return;
    const measure = () => {
      if (!viewportRef.current || !textRef.current) return;
      setDistance(Math.max(0, textRef.current.scrollWidth - viewportRef.current.clientWidth));
    };
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(viewport);
    observer.observe(text);
    void document.fonts?.ready.then(measure);
    return () => observer.disconnect();
  }, [children]);

  return (
    <span
      ref={viewportRef}
      className={cn('hover-scroll-text', className)}
      data-overflow={distance > 1 ? 'true' : undefined}
      style={{
        '--hover-scroll-distance': `${distance}px`,
        '--hover-scroll-duration': `${cycleDuration}s`,
        '--hover-scroll-motion-start': `${pauseDuration / cycleDuration * 100}%`,
        '--hover-scroll-motion-end': `${(pauseDuration + scrollDuration) / cycleDuration * 100}%`,
      } as CSSProperties}
    >
      <span key={children} ref={textRef} className="hover-scroll-text-content">{children}</span>
    </span>
  );
}
