import type { ReactNode } from 'react';
import type { LucideIcon } from 'lucide-react';
import './management.css';

export function DocumentCanvas({ title, icon: Icon, actions, footer, children }: {
  title: string; icon?: LucideIcon; actions?: ReactNode; footer?: ReactNode; children: ReactNode;
}) {
  return <div className="scrollbar-none min-h-0 min-w-0 flex-1 overflow-y-auto bg-canvas p-[clamp(1rem,3%,2rem)]" role="region" aria-label={title} tabIndex={0}>
    <article className="management-card">
      <header className="management-heading"><div className="management-title">
        {Icon && <Icon aria-hidden="true" />}<h1>{title}</h1>
      </div>{actions}</header>
      <div className="management-content">{children}</div>
      {footer && <footer className="management-footer">{footer}</footer>}
    </article>
  </div>;
}
