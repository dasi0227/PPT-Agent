import type { ReactNode } from 'react';

export function DocumentCanvas({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="scrollbar-none min-h-0 min-w-0 flex-1 overflow-y-auto bg-canvas p-[clamp(1rem,3%,2rem)]" role="region" aria-label={title} tabIndex={0}>
      <article className="mx-auto w-full max-w-[800px] rounded-lg border border-border bg-surface p-[clamp(1.25rem,4%,2.5rem)] shadow-[0_2px_8px_rgba(71,85,105,0.06)]">
        <header className="mb-7 border-b border-border pb-6">
          <h1 className="text-2xl font-semibold tracking-tight text-text-900">{title}</h1>
        </header>
        {children}
      </article>
    </div>
  );
}
