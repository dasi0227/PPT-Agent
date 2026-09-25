import type { ReactNode } from 'react';

export function DocumentSection({ title, children, compact = false }: {
  title: string;
  children: ReactNode;
  compact?: boolean;
}) {
  return (
    <section>
      <h2 className={compact ? 'mb-1 text-sm font-semibold leading-6 text-text-900' : 'mb-2 text-base font-semibold leading-6 text-text-900'}>{title}</h2>
      {children}
    </section>
  );
}

export function DocumentProperties({ items }: {
  items: { label: string; value: ReactNode; muted?: boolean }[];
}) {
  return (
    <dl className="space-y-3 text-sm leading-6">
      {items.map(({ label, value, muted }) => (
        <div key={label} className="flex flex-wrap items-baseline gap-x-6 gap-y-1">
          <dt className="w-24 shrink-0 font-medium text-text-700">{label}</dt>
          <dd className={`min-w-0 flex-[1_1_8rem] whitespace-pre-wrap break-words ${muted ? 'text-text-600' : 'text-text-900'}`}>{value}</dd>
        </div>
      ))}
    </dl>
  );
}
