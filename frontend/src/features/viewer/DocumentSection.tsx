import type { ReactNode } from 'react';

export function DocumentSection({ title, children, compact = false }: {
  title: string;
  children: ReactNode;
  compact?: boolean;
}) {
  return (
    <section>
      <h2 className={compact ? 'mb-1 text-sm font-semibold leading-6 text-text-900' : 'mb-2 text-lg font-semibold leading-7 text-text-900'}>{title}</h2>
      {children}
    </section>
  );
}
