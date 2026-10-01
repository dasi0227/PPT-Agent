import type { ReactNode } from 'react';
import { cn } from '../../lib/utils';

export function ToolbarProgressBadge({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <span
      aria-hidden="true"
      className={cn('pointer-events-none absolute -bottom-[11px] -right-[10px] whitespace-nowrap rounded-[3px] bg-panel px-0.5 text-[9px] font-semibold leading-3 tabular-nums', className)}
    >
      {children}
    </span>
  );
}
