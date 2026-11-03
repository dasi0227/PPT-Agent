import type { ButtonHTMLAttributes } from 'react';
import { cn } from '../../lib/utils';

export function Switch({ checked, onCheckedChange, className, ...props }: Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'onClick' | 'onChange'> & {
  checked: boolean; onCheckedChange: (checked: boolean) => void;
}) {
  return <button {...props} type="button" role="switch" aria-checked={checked}
    onClick={() => onCheckedChange(!checked)}
    className={cn(
      'h-5 w-9 shrink-0 rounded-full p-0.5 transition-[background-color,box-shadow,filter] enabled:hover:brightness-[.97] focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50 motion-reduce:transition-none',
      checked
        ? 'bg-success-soft shadow-[inset_0_0_0_1px_rgb(var(--ui-switch-on-border))]'
        : 'bg-panel-muted shadow-[inset_0_0_0_1px_rgb(var(--ui-border-strong))]',
      className,
    )}>
    <span aria-hidden="true" className={cn(
      'block h-4 w-4 rounded-full shadow-switch transition-[transform,background-color] motion-reduce:transition-none',
      checked ? 'translate-x-4 bg-success' : 'bg-subtle',
    )} />
  </button>;
}
