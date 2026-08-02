import React from 'react';
import { Loader2 } from 'lucide-react';
import type { RunSession } from '../../stores/runStore';

export const LiveProgressRow: React.FC<{ progress: RunSession['progress'] }> = ({ progress }) => {
  if (!progress) return null;
  return (
    <div className="flex min-h-8 items-center gap-2 px-1.5 text-xs text-text-600" aria-live="polite">
      <Loader2 className="h-3.5 w-3.5 animate-spin text-accent motion-reduce:animate-none" strokeWidth={1.75} />
      <span className="min-w-0 flex-1 truncate">{progress.text}</span>
      {progress.current !== undefined && progress.total !== undefined && (
        <span className="tabular-nums text-text-400">{progress.current} / {progress.total}</span>
      )}
    </div>
  );
};
