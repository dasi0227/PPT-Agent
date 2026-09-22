import * as React from 'react';
import * as SelectPrimitive from '@radix-ui/react-select';
import { Check, ChevronDown, ChevronUp } from 'lucide-react';
import { cn } from '../../lib/utils';
import { dropdownItemClassName, dropdownItemHighlightClassName, dropdownSurfaceClassName } from './dropdown-styles';

export interface SelectOption {
  value: string;
  label: string;
  disabled?: boolean;
}

interface SelectProps extends Omit<React.ComponentPropsWithoutRef<typeof SelectPrimitive.Trigger>, 'value' | 'defaultValue' | 'onChange' | 'children' | 'asChild'> {
  value: string;
  onValueChange: (value: string) => void;
  options: SelectOption[];
  placeholder?: string;
  name?: string;
  required?: boolean;
  /** Connect a portalled list to the surrounding editor's outside-click boundary. */
  ownerId?: string;
}

// Radix reserves the empty string for its placeholder. Encoding every option
// also supports real empty choices such as “不启用备用模型” without collisions.
const encodeValue = (value: string) => `option:${value}`;

export const Select = React.forwardRef<HTMLButtonElement, SelectProps>(({
  value,
  onValueChange,
  options,
  placeholder = '请选择',
  name,
  required,
  ownerId,
  disabled,
  className,
  ...triggerProps
}, ref) => {
  const [open, setOpen] = React.useState(false);
  const unavailable = disabled || options.length === 0;
  const selected = options.find((option) => option.value === value);

  React.useEffect(() => {
    if (unavailable) setOpen(false);
  }, [unavailable]);

  return (
    <>
      {name && <input type="hidden" name={name} value={value} disabled={disabled} />}
      <SelectPrimitive.Root
        value={selected ? encodeValue(value) : ''}
        onValueChange={(next) => onValueChange(next.slice('option:'.length))}
        open={open && !unavailable}
        onOpenChange={setOpen}
        disabled={unavailable}
        required={required}
      >
        <SelectPrimitive.Trigger
          ref={ref}
          {...triggerProps}
          className={cn(
            'ui-select-trigger inline-flex h-9 w-full min-w-0 items-center justify-between gap-2 rounded-lg border border-border bg-panel px-2.5 text-left text-[13px] font-normal text-text-800 outline-none transition-colors hover:bg-accent-soft focus-visible:bg-accent-soft data-[state=open]:bg-accent-soft data-[placeholder]:text-text-500 disabled:cursor-not-allowed disabled:opacity-50',
            className,
          )}
        >
          <span className="min-w-0 flex-1 truncate" title={selected?.label}>
            <SelectPrimitive.Value placeholder={options.length ? placeholder : '暂无可选项'}>{selected?.label}</SelectPrimitive.Value>
          </span>
          <SelectPrimitive.Icon asChild>
            <ChevronDown className={cn('h-3.5 w-3.5 shrink-0 text-text-600 transition-transform motion-reduce:transition-none', open && 'rotate-180')} strokeWidth={1.75} />
          </SelectPrimitive.Icon>
        </SelectPrimitive.Trigger>
        <SelectPrimitive.Portal>
          <SelectPrimitive.Content
            position="popper"
            side="bottom"
            align="start"
            sideOffset={6}
            collisionPadding={12}
            data-select-owner={ownerId}
            className={cn(dropdownSurfaceClassName, 'max-h-[min(18rem,var(--radix-select-content-available-height))] w-[var(--radix-select-trigger-width)] min-w-[120px] overflow-hidden')}
            onEscapeKeyDown={(event) => event.stopPropagation()}
          >
            <SelectPrimitive.ScrollUpButton className="flex h-5 items-center justify-center text-text-500">
              <ChevronUp className="h-3.5 w-3.5" aria-hidden="true" />
            </SelectPrimitive.ScrollUpButton>
            <SelectPrimitive.Viewport className="overscroll-contain">
              {options.map((option) => (
                <SelectPrimitive.Item
                  key={option.value}
                  value={encodeValue(option.value)}
                  disabled={option.disabled}
                  textValue={option.label}
                  className={cn(dropdownItemClassName, dropdownItemHighlightClassName, 'pr-8 text-[13px] data-[state=checked]:bg-accent-soft data-[state=checked]:text-accent')}
                >
                  <SelectPrimitive.ItemText>
                    <span className="block break-words [overflow-wrap:anywhere]">{option.label}</span>
                  </SelectPrimitive.ItemText>
                  <SelectPrimitive.ItemIndicator className="absolute right-2.5 inline-flex items-center text-accent">
                    <Check className="h-3.5 w-3.5" strokeWidth={1.75} />
                  </SelectPrimitive.ItemIndicator>
                </SelectPrimitive.Item>
              ))}
            </SelectPrimitive.Viewport>
            <SelectPrimitive.ScrollDownButton className="flex h-5 items-center justify-center text-text-500">
              <ChevronDown className="h-3.5 w-3.5" aria-hidden="true" />
            </SelectPrimitive.ScrollDownButton>
          </SelectPrimitive.Content>
        </SelectPrimitive.Portal>
      </SelectPrimitive.Root>
    </>
  );
});
Select.displayName = 'Select';
