import type { ReactNode } from 'react';
import type { LucideIcon } from 'lucide-react';
import './management.css';

export function DocumentCanvas({ title, icon: Icon, actions, status, footer, children }: {
  title: string; icon?: LucideIcon; actions?: ReactNode; status?: string; footer?: ReactNode; children: ReactNode;
}) {
  return <div className="document-canvas min-h-0 min-w-0 flex-1 overflow-hidden bg-canvas p-[clamp(1rem,3%,2rem)]" role="region" aria-label={title}>
    <article className="management-card">
      <header className="management-heading"><div className="management-title">
        {Icon && <Icon aria-hidden="true" />}<h1>{title}</h1>
      </div>{actions}</header>
      <div className="management-content scrollbar-none" tabIndex={0} aria-label={`${title}内容`}>{children}</div>
      {(status || footer) && <footer className="management-footer">
        {status && <div className="management-status" role="status">{status}</div>}
        {footer}
      </footer>}
    </article>
  </div>;
}
