import { useEffect, useId, useRef, type ChangeEvent } from 'react';

/** A plain labelled input is also used by the deck's existing creation dialogs. */
export function TextField({ label, value, onChange, maxLength, minLength = 0, multiline = false, autoFocus = false, describedBy, invalid, hideLabel = false }: {
  label: string; value: string; onChange: (value: string) => void; maxLength: number; minLength?: number; multiline?: boolean;
  autoFocus?: boolean; describedBy?: string; invalid?: boolean; hideLabel?: boolean;
}) {
  const id = useId();
  const input = useRef<HTMLInputElement | HTMLTextAreaElement | null>(null);
  useEffect(() => {
    input.current?.setCustomValidity([...value.trim()].length < minLength ? `请填写${label}，至少 ${minLength} 个字符。` : '');
  }, [label, minLength, value]);
  useEffect(() => { if (autoFocus) input.current?.focus(); }, [autoFocus]);
  const common = { id, value, ref: (node: HTMLInputElement | HTMLTextAreaElement | null) => { input.current = node; },
    required: minLength > 0, minLength, maxLength, 'aria-describedby': describedBy, 'aria-invalid': invalid || undefined,
    onChange: (event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => onChange(event.target.value),
    className: 'w-full min-w-0 rounded-md border border-border bg-panel px-3 py-2 text-sm text-text-900 outline-none disabled:opacity-50' };
  return <div className={hideLabel ? '' : 'space-y-2'}><label htmlFor={id} className={hideLabel ? 'sr-only' : 'block text-sm font-medium text-text-700'}>{label}</label>
    {multiline ? <textarea {...common} rows={3} className={`${common.className} h-28 resize-none overflow-y-auto`} /> : <input {...common} type="text" />}
  </div>;
}
