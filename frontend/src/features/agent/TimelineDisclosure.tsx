import type { ReactNode } from 'react';
import { cn } from '../../lib/utils';

export function TimelineDisclosure({
  open,
  children,
  className,
}: {
  open: boolean;
  children: ReactNode;
  className?: string;
}) {
  return (
    <div
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
