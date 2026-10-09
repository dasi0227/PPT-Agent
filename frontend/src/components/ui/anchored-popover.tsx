import * as React from 'react';
import * as DialogPrimitive from '@radix-ui/react-dialog';
import { cn } from '../../lib/utils';

export function AnchoredPopover(props: React.ComponentProps<typeof DialogPrimitive.Root>) {
  return <DialogPrimitive.Root {...props} modal={false} />;
}

export const AnchoredPopoverTrigger = DialogPrimitive.Trigger;
export const AnchoredPopoverTitle = DialogPrimitive.Title;

// Mount content only while open so measurements follow the current trigger.
export function AnchoredPopoverContent({
  anchorRef, align = 'start', side = 'top', sideOffset = 10, showArrow = true, viewportPadding = 12,
  viewportMode = 'visual',
  className, children, onEscapeKeyDown, onCloseAutoFocus, ...props
}: React.ComponentPropsWithoutRef<typeof DialogPrimitive.Content> & {
  anchorRef: React.RefObject<HTMLElement>;
  align?: 'start' | 'end';
  side?: 'top' | 'bottom';
  sideOffset?: number;
  showArrow?: boolean;
  viewportPadding?: number;
  viewportMode?: 'visual' | 'layout';
}) {
  const contentRef = React.useRef<HTMLDivElement>(null);
  const escaped = React.useRef(false);
  const [position, setPosition] = React.useState<{
    left: number; top: number; arrow: number; above: boolean; maxHeight: number;
  }>();

  React.useLayoutEffect(() => {
    const anchor = anchorRef.current;
    if (!anchor) return;
    anchor.scrollIntoView({ block: 'nearest', inline: 'nearest', behavior: 'instant' });
    let frame = 0;
    let observedContent: HTMLDivElement | null = null;
    const update = () => {
      const content = contentRef.current;
      if (!content) return;
      if (observedContent !== content) {
        if (observedContent) observer.unobserve(observedContent);
        observer.observe(content);
        observedContent = content;
      }
      const rect = anchor.getBoundingClientRect();
      // Layout anchoring lets pinch zoom magnify the popover with its anchor,
      // rather than moving/clipping it to the newly visible screen region.
      const viewport = viewportMode === 'visual' ? window.visualViewport : null;
      const x = viewport?.offsetLeft ?? 0;
      const y = viewport?.offsetTop ?? 0;
      const width = viewport?.width ?? window.innerWidth;
      const height = viewport?.height ?? window.innerHeight;
      const box = content.getBoundingClientRect();
      // Use un-clipped height when choosing a side to avoid flipping after a resize.
      const contentHeight = (content.firstElementChild?.scrollHeight ?? box.height) + 2;
      const anchorLeft = align === 'end' ? rect.right - box.width : rect.left;
      const left = Math.max(x + viewportPadding, Math.min(anchorLeft, x + width - box.width - viewportPadding));
      const spaceAbove = Math.max(0, rect.top - y - viewportPadding - sideOffset);
      const spaceBelow = Math.max(0, y + height - rect.bottom - viewportPadding - sideOffset);
      const above = side === 'top'
        ? spaceAbove >= contentHeight || (spaceBelow < contentHeight && spaceAbove > spaceBelow)
        : spaceBelow < contentHeight && spaceAbove > spaceBelow;
      const maxHeight = above ? spaceAbove : spaceBelow;
      const top = above ? Math.max(y + viewportPadding, rect.top - Math.min(contentHeight, maxHeight) - sideOffset) : rect.bottom + sideOffset;
      const arrow = Math.max(16, Math.min(rect.left + rect.width / 2 - left, box.width - 16));
      setPosition({ left, top, arrow, above, maxHeight });
    };
    const schedule = () => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(update);
    };
    // The portal attaches after the parent layout effect.
    const observer = new ResizeObserver(schedule);
    let ancestor: HTMLElement | null = anchor;
    while (ancestor) {
      observer.observe(ancestor);
      ancestor = ancestor.parentElement;
    }
    schedule();
    window.addEventListener('resize', schedule);
    window.addEventListener('scroll', schedule, true);
    const visualViewport = viewportMode === 'visual' ? window.visualViewport : null;
    visualViewport?.addEventListener('resize', schedule);
    visualViewport?.addEventListener('scroll', schedule);
    return () => {
      cancelAnimationFrame(frame);
      observer.disconnect();
      window.removeEventListener('resize', schedule);
      window.removeEventListener('scroll', schedule, true);
      visualViewport?.removeEventListener('resize', schedule);
      visualViewport?.removeEventListener('scroll', schedule);
    };
  }, [anchorRef, align, side, sideOffset, viewportPadding, viewportMode]);

  return (
    <DialogPrimitive.Portal>
      <DialogPrimitive.Content
        {...props}
        ref={contentRef}
        aria-describedby={undefined}
        className={cn('fixed z-50 w-[280px] max-w-[calc(100vw-24px)] rounded-[10px] border border-border bg-surface text-text-900 shadow-overlay outline-none', className)}
        style={{ left: position?.left, top: position?.top, maxHeight: position?.maxHeight, opacity: position ? 1 : 0, ...props.style }}
        onEscapeKeyDown={(event) => {
          onEscapeKeyDown?.(event);
          escaped.current = !event.defaultPrevented;
          event.stopPropagation();
        }}
        onCloseAutoFocus={(event) => {
          onCloseAutoFocus?.(event);
          if (escaped.current) event.preventDefault();
        }}
      >
        <div className="max-h-[calc(100dvh-48px)] overflow-y-auto overscroll-contain rounded-[inherit]" style={{ maxHeight: position ? Math.max(0, position.maxHeight - 2) : undefined }}>{children}</div>
        {showArrow && position && (
          <span aria-hidden="true"
            className={cn('pointer-events-none absolute h-2 w-2 rotate-45 bg-surface', position.above ? '-bottom-[5px] border-b border-r border-border' : '-top-[5px] border-l border-t border-border')}
            style={{ left: position.arrow - 4 }}
          />
        )}
      </DialogPrimitive.Content>
    </DialogPrimitive.Portal>
  );
}
