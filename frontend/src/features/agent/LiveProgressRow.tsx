import React from 'react';
import type { RunSession } from '../../stores/runStore';
import { B2Orb } from './B2Orb';
import { runActivityLabels } from './runtimeLabels';

export const LiveProgressRow: React.FC<{ progress: RunSession['progress'] }> = ({ progress }) => {
  if (!progress) return null;
  const label = runActivityLabels[progress.activity];
  return (
    <div className="flex min-h-8 items-center gap-2 px-1.5 text-xs text-text-600" aria-live="polite">
      <B2Orb className="text-accent" label={label} />
      <span className="timeline-loading-shimmer min-w-0 flex-1 truncate">{label}</span>
    </div>
  );
};
