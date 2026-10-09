import type { ReactNode } from 'react';
import { cn } from '../../lib/utils';
import { ChevronRight } from 'lucide-react';

export function TimelineChevron({ open, className }: { open: boolean; className?: string }) {
  return <ChevronRight data-open={open} aria-hidden="true" strokeWidth={1.75}
    className={cn('timeline-chevron h-3.5 w-3.5 shrink-0 text-text-400', className)} />;
}

export function TimelineDisclosure({
  open,
  children,
  className,
  id,
}: {
  open: boolean;
  children: ReactNode;
  className?: string;
  id?: string;
}) {
  return (
    <div
      id={id}
      ref={(node) => { if (node) node.inert = !open; }}
      data-state={open ? 'open' : 'closed'}
      aria-hidden={!open}
      className={cn('timeline-disclosure', className)}
    >
      <div className="timeline-disclosure-clip">
        <div className="timeline-disclosure-content">
          {children}
        </div>
      </div>
    </div>
  );
}
