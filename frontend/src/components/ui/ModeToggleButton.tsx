import type { LucideIcon } from 'lucide-react';
import { cn } from '../../lib/utils';

type ModeOption = { value: string; label: string; icon: LucideIcon };

/** Keep one persistent selection surface so both pointer and shortcut changes animate. */
export function ModeToggleButton({ label, value, options, onValueChange }: {
  label: string;
  value: string;
  options: readonly [ModeOption, ModeOption];
  onValueChange: (value: string) => void;
}) {
  const selectedIndex = options.findIndex(option => option.value === value);
  return (
    <div role="group" aria-label={label} className="relative isolate grid h-8 shrink-0 grid-cols-2 rounded-full bg-panel-muted p-0.5 text-xs">
      <span
        aria-hidden="true"
        className="pointer-events-none absolute bottom-0.5 left-0.5 top-0.5 -z-10 w-[calc(50%-2px)] rounded-full bg-accent-soft transition-transform duration-200 ease-out motion-reduce:transition-none"
        style={{ transform: `translateX(${selectedIndex === 1 ? 100 : 0}%)` }}
      />
      {options.map(({ value: optionValue, label: optionLabel, icon: Icon }) => (
        <button
          key={optionValue}
          type="button"
          aria-pressed={value === optionValue}
          onClick={() => onValueChange(optionValue)}
          className={cn(
            'inline-flex min-w-0 items-center justify-center gap-1.5 whitespace-nowrap rounded-full px-3 font-medium transition-colors duration-200 hover:bg-accent-soft hover:text-accent focus-visible:bg-accent-soft focus-visible:text-accent focus-visible:outline-none motion-reduce:transition-none',
            value === optionValue ? 'text-accent' : 'text-text-600',
          )}
        >
          <Icon className="h-3.5 w-3.5 shrink-0" strokeWidth={1.75} aria-hidden="true" />
          <span>{optionLabel}</span>
        </button>
      ))}
    </div>
  );
}
