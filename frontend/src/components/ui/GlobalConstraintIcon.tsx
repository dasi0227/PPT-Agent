import { forwardRef } from 'react';
import { Ruler, type LucideProps } from 'lucide-react';

export const GlobalConstraintIcon = forwardRef<SVGSVGElement, LucideProps>(
  ({ style, ...props }, ref) => (
    <Ruler ref={ref} {...props} style={{ ...style, transform: 'rotate(-90deg)' }} />
  ),
);
GlobalConstraintIcon.displayName = 'GlobalConstraintIcon';
