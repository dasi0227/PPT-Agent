import React from 'react';
import { cn } from '../../lib/utils';

interface B2OrbProps {
  className?: string;
  label?: string;
}

// Geometry and timing adapted from AICSS's MIT-licensed B2 "drift" Orb.
export const B2Orb: React.FC<B2OrbProps> = ({ className, label = 'Agent 正在处理' }) => (
  <span
    role="img"
    aria-label={label}
    className={cn('agent-b2-orb', className)}
  >
    <span className="agent-b2-orb-stage" aria-hidden="true">
      <span className="agent-b2-orb-shape" />
      <span className="agent-b2-orb-shape" />
      <span className="agent-b2-orb-shape" />
    </span>
  </span>
);
