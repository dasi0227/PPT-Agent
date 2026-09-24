import { useCallback, useLayoutEffect, useRef } from 'react';
import type { LucideIcon } from 'lucide-react';
import './ModeToggleButton.css';

type ModeOption = { value: string; label: string; icon: LucideIcon };
type Drag = { pointerId: number; startX: number; startPosition: number; position: number; moved: boolean };

/** A controlled two-mode slider; the visible label always names the current mode. */
export function ModeToggleButton({ label, value, options, onValueChange, disabled = false }: {
  label: string;
  value: string;
  options: readonly [ModeOption, ModeOption];
  onValueChange: (value: string) => void;
  disabled?: boolean;
}) {
  const selectedIndex = value === options[1].value ? 1 : 0;
  const buttonRef = useRef<HTMLButtonElement>(null);
  const thumbRef = useRef<HTMLSpanElement>(null);
  const dragRef = useRef<Drag | null>(null);
  const suppressClickRef = useRef(false);
  const travel = useCallback(() => {
    const button = buttonRef.current;
    const thumb = thumbRef.current;
    if (!button || !thumb) return 0;
    return Math.max(0, button.clientWidth - thumb.offsetWidth - 2 * thumb.offsetLeft);
  }, []);
  const paint = useCallback((position: number) => {
    const button = buttonRef.current;
    if (!button) return;
    button.style.setProperty('--mode-thumb-x', `${position}px`);
    button.dataset.side = position > travel() / 2 ? 'right' : 'left';
  }, [travel]);

  // External shortcuts and document/empty states must also settle an active drag.
  useLayoutEffect(() => {
    const button = buttonRef.current;
    const drag = dragRef.current;
    dragRef.current = null;
    if (button) {
      delete button.dataset.dragging;
      if (drag && button.hasPointerCapture(drag.pointerId)) button.releasePointerCapture(drag.pointerId);
    }
    paint(selectedIndex * travel());
  }, [selectedIndex, disabled, paint, travel]);

  const commit = (index: number) => {
    if (disabled) return;
    paint(index * travel());
    if (index !== selectedIndex) onValueChange(options[index].value);
  };
  const cancelDrag = () => {
    if (!dragRef.current) return;
    dragRef.current = null;
    if (buttonRef.current) delete buttonRef.current.dataset.dragging;
    paint(selectedIndex * travel());
  };

  return (
    <button
      ref={buttonRef}
      type="button"
      role="slider"
      aria-label={label}
      aria-valuemin={0}
      aria-valuemax={1}
      aria-valuenow={selectedIndex}
      aria-valuetext={options[selectedIndex].label}
      aria-orientation="horizontal"
      disabled={disabled}
      title={disabled ? `${label}：${options[selectedIndex].label}，当前不可切换` : `切换为${options[1 - selectedIndex].label}`}
      className="mode-toggle group bg-transparent text-text-600 enabled:hover:text-accent focus-visible:text-accent focus-visible:outline-none disabled:cursor-not-allowed disabled:text-text-400"
      onClick={event => {
        if (suppressClickRef.current && event.detail !== 0) {
          suppressClickRef.current = false;
          return;
        }
        suppressClickRef.current = false;
        commit(1 - selectedIndex);
      }}
      onKeyDown={event => {
        if (event.metaKey || event.ctrlKey || event.altKey || event.shiftKey) return;
        if (!['ArrowLeft', 'ArrowRight', 'ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return;
        event.preventDefault();
        event.stopPropagation();
        commit(['ArrowRight', 'ArrowUp', 'End'].includes(event.key) ? 1 : 0);
      }}
      onPointerDown={event => {
        if (disabled || !event.isPrimary || event.button !== 0) return;
        suppressClickRef.current = false;
        event.currentTarget.focus({ preventScroll: true });
        const position = thumbRef.current
          ? new DOMMatrixReadOnly(getComputedStyle(thumbRef.current).transform).m41
          : selectedIndex * travel();
        dragRef.current = { pointerId: event.pointerId, startX: event.clientX, startPosition: position, position, moved: false };
        event.currentTarget.setPointerCapture(event.pointerId);
      }}
      onPointerMove={event => {
        const drag = dragRef.current;
        if (disabled || !drag || drag.pointerId !== event.pointerId) return;
        const delta = event.clientX - drag.startX;
        if (Math.abs(delta) > 4) drag.moved = true;
        if (!drag.moved) return;
        event.currentTarget.dataset.dragging = 'true';
        drag.position = Math.max(0, Math.min(travel(), drag.startPosition + delta));
        paint(drag.position);
      }}
      onPointerUp={event => {
        const drag = dragRef.current;
        if (!drag || drag.pointerId !== event.pointerId) return;
        dragRef.current = null;
        delete event.currentTarget.dataset.dragging;
        if (drag.moved) {
          suppressClickRef.current = true;
          commit(drag.position > travel() / 2 ? 1 : 0);
        }
        if (event.currentTarget.hasPointerCapture(event.pointerId)) event.currentTarget.releasePointerCapture(event.pointerId);
      }}
      onPointerCancel={cancelDrag}
      onLostPointerCapture={cancelDrag}
    >
      {options.map(({ value: optionValue, label: optionLabel, icon: Icon }, index) => (
        <span key={optionValue} aria-hidden="true" className={`mode-toggle-label mode-toggle-label-${index === 0 ? 'left' : 'right'}`}>
          <Icon className="h-3.5 w-3.5 shrink-0" strokeWidth={1.75} />
          <span>{optionLabel}</span>
        </span>
      ))}
      <span ref={thumbRef} aria-hidden="true" className="mode-toggle-thumb bg-current opacity-25 group-disabled:opacity-20" />
    </button>
  );
}
