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
  anchorRef, className, children, onEscapeKeyDown, onCloseAutoFocus, ...props
}: React.ComponentPropsWithoutRef<typeof DialogPrimitive.Content> & {
  anchorRef: React.RefObject<HTMLElement>;
}) {
  const contentRef = React.useRef<HTMLDivElement>(null);
  const escaped = React.useRef(false);
  const [position, setPosition] = React.useState<{
    left: number; top: number; arrow: number; above: boolean;
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
      const viewport = window.visualViewport;
      const x = viewport?.offsetLeft ?? 0;
      const y = viewport?.offsetTop ?? 0;
      const width = viewport?.width ?? window.innerWidth;
      const height = viewport?.height ?? window.innerHeight;
      const box = content.getBoundingClientRect();
      const left = Math.max(x + 12, Math.min(rect.left, x + width - box.width - 12));
      const above = rect.top - y >= box.height + 22 || rect.bottom + box.height + 22 > y + height;
      const top = above ? Math.max(y + 12, rect.top - box.height - 10) : rect.bottom + 10;
      const arrow = Math.max(16, Math.min(rect.left + rect.width / 2 - left, box.width - 16));
      setPosition({ left, top, arrow, above });
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
    window.visualViewport?.addEventListener('resize', schedule);
    window.visualViewport?.addEventListener('scroll', schedule);
    return () => {
      cancelAnimationFrame(frame);
      observer.disconnect();
      window.removeEventListener('resize', schedule);
      window.removeEventListener('scroll', schedule, true);
      window.visualViewport?.removeEventListener('resize', schedule);
      window.visualViewport?.removeEventListener('scroll', schedule);
    };
  }, [anchorRef]);

  return (
    <DialogPrimitive.Portal>
      <DialogPrimitive.Content
        {...props}
        ref={contentRef}
        aria-describedby={undefined}
        className={cn('fixed z-50 w-[280px] max-w-[calc(100vw-24px)] rounded-[10px] border border-border bg-surface text-text-900 shadow-[0_6px_20px_rgba(23,32,43,0.09)] outline-none', className)}
        style={{ left: position?.left, top: position?.top, opacity: position ? 1 : 0, ...props.style }}
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
        <div className="max-h-[calc(100dvh-48px)] overflow-y-auto overscroll-contain rounded-[inherit]">{children}</div>
        {position && (
          <span aria-hidden="true"
            className={cn('pointer-events-none absolute h-2 w-2 rotate-45 bg-surface', position.above ? '-bottom-[5px] border-b border-r border-border' : '-top-[5px] border-l border-t border-border')}
            style={{ left: position.arrow - 4 }}
          />
        )}
      </DialogPrimitive.Content>
    </DialogPrimitive.Portal>
  );
}
