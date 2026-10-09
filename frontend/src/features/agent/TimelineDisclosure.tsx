import type { ReactNode } from 'react';
import { cn } from '../../lib/utils';

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
