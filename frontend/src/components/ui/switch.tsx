import type { ButtonHTMLAttributes } from 'react';
import { cn } from '../../lib/utils';

export function Switch({ checked, onCheckedChange, className, ...props }: Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'onClick' | 'onChange'> & {
  checked: boolean; onCheckedChange: (checked: boolean) => void;
}) {
  return <button {...props} type="button" role="switch" aria-checked={checked}
    onClick={() => onCheckedChange(!checked)}
    className={cn('h-5 w-9 shrink-0 rounded-full p-0.5 transition-colors focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50 motion-reduce:transition-none', checked ? 'bg-accent' : 'bg-border-strong', className)}>
    <span aria-hidden="true" className={cn('block h-4 w-4 rounded-full bg-white shadow-switch transition-transform motion-reduce:transition-none', checked && 'translate-x-4')} />
  </button>;
}
