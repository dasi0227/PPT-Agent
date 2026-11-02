import { forwardRef } from 'react';
import { Ruler, type LucideProps } from 'lucide-react';

export const GlobalConstraintsIcon = forwardRef<SVGSVGElement, LucideProps>(
  ({ style, ...props }, ref) => (
    <Ruler ref={ref} {...props} style={{ ...style, transform: 'rotate(-90deg)' }} />
  ),
);
GlobalConstraintsIcon.displayName = 'GlobalConstraintsIcon';
