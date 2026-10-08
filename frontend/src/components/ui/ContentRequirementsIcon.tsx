import { forwardRef } from 'react';
import { PenTool, type LucideProps } from 'lucide-react';

export const ContentRequirementsIcon = forwardRef<SVGSVGElement, LucideProps>(
  ({ style, ...props }, ref) => (
    <PenTool ref={ref} {...props} style={{ ...style, transform: 'rotate(-90deg)' }} />
  ),
);
ContentRequirementsIcon.displayName = 'ContentRequirementsIcon';
