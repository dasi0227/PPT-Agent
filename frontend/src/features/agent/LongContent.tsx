import { useId, useLayoutEffect, useRef, useState, type ReactNode } from 'react';
import { ChevronDown, ChevronUp } from 'lucide-react';
import { cn } from '../../lib/utils';

const DEFAULT_MAX_HEIGHT = 320;

export function LongContent({
  children,
  className,
  contentClassName,
  fadeClassName,
  buttonClassName,
  controlsClassName,
  maxHeight = DEFAULT_MAX_HEIGHT,
  testId,
}: {
  children: ReactNode;
  className?: string;
  contentClassName?: string;
  fadeClassName?: string;
  buttonClassName?: string;
  controlsClassName?: string;
  maxHeight?: number;
  testId?: string;
}) {
  const [expanded, setExpanded] = useState(false);
  const [overflowing, setOverflowing] = useState(false);
  const viewportRef = useRef<HTMLDivElement>(null);
  const contentRef = useRef<HTMLDivElement>(null);
  const contentId = useId();

  useLayoutEffect(() => {
    const viewport = viewportRef.current;
    const content = contentRef.current;
    if (!viewport || !content) return undefined;

    const measure = () => {
      if (expanded) return;
      setOverflowing(viewport.scrollHeight - viewport.clientHeight > 1);
    };

    measure();
    if (typeof ResizeObserver === 'undefined') return undefined;
    const observer = new ResizeObserver(measure);
    observer.observe(content);
    return () => observer.disconnect();
  }, [children, expanded, maxHeight]);

  const controlClassName = cn(
    'inline-flex h-7 items-center justify-center gap-1 rounded-lg border border-border bg-surface px-3 text-xs font-medium text-text-600 shadow-sm transition-colors hover:bg-panel-muted hover:text-text-900 focus-visible:outline-none',
    buttonClassName,
  );

  return (
    <div className={className} data-expanded={expanded} data-overflow={overflowing}>
      <div
        ref={viewportRef}
        data-testid={testId}
        className={cn('relative', !expanded && 'overflow-hidden')}
        style={expanded ? undefined : { maxHeight }}
      >
        <div ref={contentRef} id={contentId} className={contentClassName}>
          {children}
        </div>
        {!expanded && overflowing && (
          <div
            className={cn(
              'absolute inset-x-0 bottom-0 flex h-20 items-end justify-center bg-gradient-to-b from-surface/0 via-surface/90 to-surface pb-2 backdrop-blur-[1px]',
              fadeClassName,
            )}
          >
            <button
              type="button"
              aria-expanded="false"
              aria-controls={contentId}
              onClick={(event) => {
                event.stopPropagation();
                setExpanded(true);
              }}
              className={controlClassName}
            >
              <ChevronDown className="h-3.5 w-3.5" strokeWidth={1.75} aria-hidden="true" />
              展开全部
            </button>
          </div>
        )}
      </div>
      {expanded && overflowing && (
        <div className={cn('mt-3 flex justify-center', controlsClassName)}>
          <button
            type="button"
            aria-expanded="true"
            aria-controls={contentId}
            onClick={(event) => {
              event.stopPropagation();
              setExpanded(false);
            }}
            className={controlClassName}
          >
            <ChevronUp className="h-3.5 w-3.5" strokeWidth={1.75} aria-hidden="true" />
            收起
          </button>
        </div>
      )}
    </div>
  );
}
