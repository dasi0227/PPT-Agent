import React from 'react';
import { Check, Clipboard } from 'lucide-react';
import { cn } from '../../lib/utils';
import { formatTimestamp } from '../../lib/formatTimestamp';

export function MessageMetaActions({
  text,
  timestamp,
  label,
  scopeLabel,
  children,
}: {
  text: string;
  timestamp: number;
  label: string;
  scopeLabel?: string;
  children?: React.ReactNode;
}) {
  const [copied, setCopied] = React.useState(false);

  const copy = async () => {
    await navigator.clipboard?.writeText(text);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1200);
  };

  return (
    <div className={cn(
      'mt-1 inline-flex items-center gap-1 text-text-400 transition-opacity duration-150 motion-reduce:transition-none',
      copied ? 'opacity-100' : 'opacity-0 group-hover:opacity-100 group-focus-within:opacity-100',
    )}>
      <button
        type="button"
        onClick={() => void copy()}
        className="inline-flex h-6 w-6 items-center justify-center rounded-md hover:bg-panel-muted hover:text-text-900"
        aria-label={label}
        title={copied ? '已复制' : label}
      >
        {copied ? <Check className="h-3.5 w-3.5" /> : <Clipboard className="h-3.5 w-3.5" />}
      </button>
      {children}
      {scopeLabel && <span className="select-none whitespace-nowrap text-[11px]">{scopeLabel}</span>}
      <time className="select-none whitespace-nowrap text-[11px] tabular-nums" dateTime={new Date(timestamp).toISOString()}>
        {formatTimestamp(timestamp)}
      </time>
    </div>
  );
}
