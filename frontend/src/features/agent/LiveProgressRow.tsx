import React from 'react';
import type { RunSession } from '../../stores/runStore';
import { B2Orb } from './B2Orb';
import { runActivityLabels } from './runtimeLabels';

export const LiveProgressRow: React.FC<{ progress: RunSession['progress'] }> = ({ progress }) => {
  if (!progress) return null;
  const label = runActivityLabels[progress.activity];
  if (!label) return null;
  return (
    <div className="grid min-h-8 grid-cols-[16px_minmax(0,1fr)_16px] items-center gap-2 px-1.5 py-1 text-[13px] font-normal text-text-600" aria-live="polite">
      <B2Orb className="text-accent" label={label} />
      <span className="timeline-loading-shimmer min-w-0 truncate">{label}</span>
    </div>
  );
};
