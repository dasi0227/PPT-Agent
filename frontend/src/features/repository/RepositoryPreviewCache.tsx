import { type ReactNode, useEffect, useState } from 'react';
import { cn } from '../../lib/utils';

interface PreviewEntry {
  key: string;
  content: ReactNode;
}

// Keep visited documents connected: detaching an iframe discards its execution.
// Nine entries cover the three built-in themes × three examples. Bound custom repositories too.
export function RepositoryPreviewCache({ entries, activeKey }: { entries: PreviewEntry[]; activeKey: string }) {
  const [visited, setVisited] = useState<string[]>([]);
  const available = new Set(entries.map(entry => entry.key));
  const retained = [...visited.filter(key => key !== activeKey && available.has(key)), activeKey].slice(-9);
  const signature = JSON.stringify(retained);
  useEffect(() => { setVisited(JSON.parse(signature) as string[]); }, [signature]);

  return (
    <div className="relative h-full w-full">
      {entries.filter(entry => retained.includes(entry.key)).map(entry => (
        <div
          key={entry.key}
          data-preview-active={entry.key === activeKey}
          aria-hidden={entry.key !== activeKey || undefined}
          {...(entry.key !== activeKey ? { inert: '' } : {})}
          className={cn('absolute inset-0', entry.key !== activeKey && 'invisible pointer-events-none')}
        >
          {entry.content}
        </div>
      ))}
    </div>
  );
}
