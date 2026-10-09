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
  horizontalScroll = false,
  hideScrollbar = false,
  maxHeight = DEFAULT_MAX_HEIGHT,
  testId,
  controlsPlacement = 'overlay',
  expandLabel = '展开',
}: {
  children: ReactNode;
  className?: string;
  contentClassName?: string;
  fadeClassName?: string;
  buttonClassName?: string;
  controlsClassName?: string;
  horizontalScroll?: boolean;
  hideScrollbar?: boolean;
  maxHeight?: number;
  testId?: string;
  controlsPlacement?: 'overlay' | 'below';
  expandLabel?: string;
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

  // Fade the content itself so text cannot show through the expansion control.
  // Keep the scrollbar outside this mask for horizontal source/output scrolling.
  const contentMask = !expanded && overflowing
    ? `linear-gradient(to bottom, #000 ${Math.max(0, maxHeight - 80)}px, transparent ${Math.max(0, maxHeight - 40)}px)`
    : undefined;

  const controlClassName = cn(
    'pointer-events-auto inline-flex h-7 items-center justify-center gap-1 rounded-lg border border-border bg-surface px-3 text-xs font-medium text-text-600 shadow-sm transition-colors ui-interactive focus-visible:outline-none',
    buttonClassName,
  );

  return (
    <div className={className} data-expanded={expanded} data-overflow={overflowing}>
      <div
        className="relative isolate"
      >
        <div
          ref={viewportRef}
          data-testid={testId}
          tabIndex={horizontalScroll ? 0 : undefined}
          className={cn(horizontalScroll ? 'overflow-x-auto overflow-y-hidden' : !expanded && 'overflow-hidden', horizontalScroll && (hideScrollbar ? '[scrollbar-width:none] [&::-webkit-scrollbar]:hidden' : '[scrollbar-width:thin]'))}
          style={expanded ? undefined : { maxHeight }}
        >
          <div ref={contentRef} id={contentId} className={contentClassName} style={{ maskImage: contentMask, WebkitMaskImage: contentMask }}>
            {children}
          </div>
        </div>
        {!expanded && overflowing && (
          <div
            className={cn(
              'pointer-events-none absolute inset-x-0 z-10 bottom-0 flex h-20 items-end justify-center bg-gradient-to-b from-surface/0 via-surface/90 to-surface pb-2',
              horizontalScroll && !hideScrollbar && 'bottom-3',
              fadeClassName,
            )}
          >
            {controlsPlacement === 'overlay' && <button
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
              {expandLabel}
            </button>}
          </div>
        )}
      </div>
      {(expanded || controlsPlacement === 'below') && overflowing && (
        <div className={cn('mt-3 flex justify-center', controlsClassName)}>
          <button
            type="button"
            aria-expanded={expanded}
            aria-controls={contentId}
            onClick={(event) => {
              event.stopPropagation();
              setExpanded(!expanded);
            }}
            className={controlClassName}
          >
            {expanded
              ? <ChevronUp className="h-3.5 w-3.5" strokeWidth={1.75} aria-hidden="true" />
              : <ChevronDown className="h-3.5 w-3.5" strokeWidth={1.75} aria-hidden="true" />}
            {expanded ? '收起' : expandLabel}
          </button>
        </div>
      )}
    </div>
  );
}
