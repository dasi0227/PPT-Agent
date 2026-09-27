import type { ReactNode } from 'react';

export const timelineCardActionClass = 'ui-interactive inline-flex shrink-0 items-center gap-1 rounded-md px-2 py-1.5 text-[11px] font-normal text-text-600 disabled:cursor-default disabled:opacity-50';

export function TimelineCardHeader({ children, action }: { children: ReactNode; action: ReactNode }) {
  return <header className="mx-3.5 flex min-h-[42px] items-start justify-between gap-2.5 border-b border-[#D7DCE3] py-2 text-xs font-normal text-text-600">
    {children}
    {action}
  </header>;
}
