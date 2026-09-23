import { useEffect, useState } from 'react';
import { LoaderCircle } from 'lucide-react';
import { cn } from '../../lib/utils';

export function PreviewLoading({ label = '正在加载预览', miniature = false }: { label?: string; miniature?: boolean }) {
  const [visible, setVisible] = useState(false);
  useEffect(() => {
    const timer = window.setTimeout(() => setVisible(true), 180);
    return () => window.clearTimeout(timer);
  }, []);
  if (!visible) return null;
  return (
    <div role="status" aria-label={label} className="pointer-events-none absolute inset-0 grid place-items-center">
      <LoaderCircle
        aria-hidden="true"
        strokeWidth={1.75}
        className={cn('animate-spin text-text-400 motion-reduce:animate-none', miniature ? 'h-3 w-3' : 'h-7 w-7')}
      />
    </div>
  );
}
