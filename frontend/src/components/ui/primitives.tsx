import React from 'react';
import { ChevronDown } from 'lucide-react';
import { cn } from '../../lib/utils';

type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger';

export function Button({
  variant = 'secondary',
  className,
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement> & { variant?: ButtonVariant }) {
  return (
    <button
      {...props}
      className={cn(
        'inline-flex h-8 items-center justify-center gap-1.5 whitespace-nowrap rounded-md px-3 text-sm font-medium disabled:cursor-not-allowed disabled:opacity-45',
        variant === 'primary' && 'bg-accent text-white hover:bg-accent/90',
        variant === 'secondary' && 'border border-border-strong bg-panel text-text-900 hover:bg-panel-muted',
        variant === 'ghost' && 'text-text-600 hover:bg-panel-muted hover:text-text-900',
        variant === 'danger' && 'bg-danger text-white hover:bg-danger/90',
        className,
      )}
    />
  );
}

export function IconButton({
  label,
  className,
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement> & { label: string }) {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      {...props}
      className={cn(
        'inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-md text-text-600 hover:bg-panel-muted hover:text-text-900 disabled:cursor-not-allowed disabled:opacity-40',
        className,
      )}
    />
  );
}

export function Badge({
  tone = 'neutral',
  className,
  children,
}: {
  tone?: 'neutral' | 'accent' | 'success' | 'warning' | 'danger';
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <span className={cn(
      'inline-flex items-center rounded-full border px-2 py-0.5 text-[11px] font-medium',
      tone === 'neutral' && 'border-border bg-panel-muted text-text-600',
      tone === 'accent' && 'border-accent/20 bg-accent-soft text-accent',
      tone === 'success' && 'border-success/20 bg-success-soft text-success',
      tone === 'warning' && 'border-warning/20 bg-warning-soft text-warning',
      tone === 'danger' && 'border-danger/20 bg-danger-soft text-danger',
      className,
    )}>{children}</span>
  );
}

export function InlineNotice({
  tone,
  children,
  className,
}: {
  tone: 'info' | 'warning' | 'danger';
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div
      role={tone === 'danger' ? 'alert' : 'status'}
      className={cn(
        'rounded-lg border px-3 py-2 text-sm',
        tone === 'info' && 'border-accent/20 bg-accent-soft text-text-900',
        tone === 'warning' && 'border-warning/25 bg-warning-soft text-text-900',
        tone === 'danger' && 'border-danger/25 bg-danger-soft text-danger',
        className,
      )}
    >{children}</div>
  );
}

export function Skeleton({ className }: { className?: string }) {
  return <div aria-hidden="true" className={cn('animate-pulse rounded bg-border/70', className)} />;
}

export function Disclosure({
  label,
  children,
  defaultOpen = false,
}: {
  label: string;
  children: React.ReactNode;
  defaultOpen?: boolean;
}) {
  return (
    <details open={defaultOpen} className="group rounded-md border border-border bg-panel-muted/60">
      <summary className="flex cursor-pointer list-none items-center gap-1.5 px-3 py-2 text-xs font-medium text-text-600 hover:text-text-900">
        <ChevronDown className="h-3.5 w-3.5 -rotate-90 transition-transform group-open:rotate-0" />
        {label}
      </summary>
      <div className="border-t border-border px-3 py-2">{children}</div>
    </details>
  );
}
