import { useCallback, useEffect, useId, useLayoutEffect, useRef, useState, type PointerEvent } from 'react';
import { FileImage, Hash, Heading1, Heading2, Loader2, RotateCcw, Undo2, type LucideIcon } from 'lucide-react';
import type { DecorationPlacement, Decorations, DecorationType } from '../../api/types';
import { AnchoredPopoverTitle } from '../../components/ui/anchored-popover';
import { floatingPanelTitleClassName } from '../../components/ui/dropdown-styles';
import { IconButton } from '../../components/ui/primitives';
import { cn } from '../../lib/utils';
import { decorationTypeLabel } from './semanticLabels';
import './decorationPlacement.css';

const fields: DecorationType[] = ['slide_pagination', 'deck_title', 'section_title', 'logo'];
const positions: { value: DecorationPlacement; label: string }[] = [
  { value: 'top-left', label: '左上' }, { value: 'top-center', label: '上中' }, { value: 'top-right', label: '右上' },
  { value: 'bottom-left', label: '左下' }, { value: 'bottom-center', label: '下中' }, { value: 'bottom-right', label: '右下' },
];
const poolSlots = ['pool-0', 'pool-1', 'pool-2', 'pool-3'] as const;
type Slot = DecorationPlacement | typeof poolSlots[number];
type Layout = Record<Slot, DecorationType | null>;
const allSlots: Slot[] = [...positions.map(position => position.value), ...poolSlots];
const icons: Record<DecorationType, LucideIcon> = {
  slide_pagination: Hash, deck_title: Heading1, section_title: Heading2, logo: FileImage,
};
const defaults: Decorations = { slide_pagination: 'bottom-right', deck_title: 'none', section_title: 'top-left', logo: 'none' };

function makeLayout(value: Decorations, previous?: Layout): Layout {
  const layout = Object.fromEntries(allSlots.map(slot => [slot, null])) as Layout;
  const hidden = new Set(fields.filter(field => value[field] === 'none'));
  for (const field of fields) {
    const position = value[field];
    if (position !== 'none') layout[position] = field;
  }
  for (const slot of poolSlots) {
    const field = previous?.[slot];
    if (field && hidden.delete(field)) layout[slot] = field;
  }
  for (const field of hidden) {
    const slot = poolSlots.find(key => !layout[key]);
    if (slot) layout[slot] = field;
  }
  return layout;
}

function toPositions(layout: Layout): Decorations {
  const value: Decorations = { slide_pagination: 'none', deck_title: 'none', section_title: 'none', logo: 'none' };
  for (const position of positions) {
    const field = layout[position.value];
    if (field) value[field] = position.value;
  }
  return value;
}

function samePositions(a: Decorations, b: Decorations) { return fields.every(field => a[field] === b[field]); }

type Drag = {
  field: DecorationType; source: Slot; pointerId: number; x: number; y: number;
  offsetX: number; offsetY: number; rect: DOMRect; button: HTMLButtonElement; tile: HTMLSpanElement;
  ghost: HTMLElement | null;
};

function disposeDrag(drag: Drag | null) {
  drag?.ghost?.remove();
  if (drag?.button.hasPointerCapture(drag.pointerId)) drag.button.releasePointerCapture(drag.pointerId);
  document.body.classList.remove('decorations-dragging');
}

export function DecorationPlacementEditor({ value, disabled, loading, blocked, onChange }: {
  value: Decorations;
  disabled: boolean;
  loading: boolean;
  blocked?: string;
  onChange: (value: Decorations) => Promise<boolean>;
}) {
  const poolTitleId = useId();
  const rootRef = useRef<HTMLDivElement>(null);
  const poolRef = useRef<HTMLDivElement>(null);
  const tileRefs = useRef(new Map<DecorationType, HTMLSpanElement>());
  const slotRefs = useRef(new Map<Slot, HTMLButtonElement>());
  const beforeRects = useRef<Map<DecorationType, DOMRect> | null>(null);
  const animations = useRef<Animation[]>([]);
  const dragRef = useRef<Drag | null>(null);
  const ignoreClickUntil = useRef(0);
  const pendingRef = useRef(false);
  const mountedRef = useRef(false);
  const focusField = useRef<DecorationType | null>(null);
  const [layout, setLayout] = useState(() => makeLayout(value));
  const [history, setHistory] = useState<Layout[]>([]);
  const [selected, setSelected] = useState<DecorationType | null>(null);
  const [dragged, setDragged] = useState<DecorationType | null>(null);
  const [target, setTarget] = useState<Slot | null>(null);
  const [pending, setPending] = useState(false);
  const layoutRef = useRef(layout);
  const valueRef = useRef(value);
  valueRef.current = value;
  const unavailable = disabled || pending;

  const stopDrag = useCallback(() => {
    const drag = dragRef.current;
    dragRef.current = null;
    if (drag?.ghost) ignoreClickUntil.current = performance.now() + 250;
    disposeDrag(drag);
    setDragged(null);
    setTarget(null);
  }, []);

  const updateLayout = useCallback((next: Layout) => {
    beforeRects.current = new Map([...tileRefs.current].map(([field, tile]) => [field, tile.getBoundingClientRect()]));
    animations.current.forEach(animation => animation.cancel());
    animations.current = [];
    layoutRef.current = next;
    setLayout(next);
  }, []);

  useLayoutEffect(() => {
    const before = beforeRects.current;
    beforeRects.current = null;
    if (before && !window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
      for (const [field, tile] of tileRefs.current) {
        const start = before.get(field);
        const end = tile.getBoundingClientRect();
        if (!start?.width || !end.width || (Math.abs(start.x - end.x) < 1 && Math.abs(start.y - end.y) < 1)) continue;
        animations.current.push(tile.animate([
          { transform: `translate(${start.x - end.x}px, ${start.y - end.y}px) scale(${start.width / end.width}, ${start.height / end.height})` },
          { transform: 'none' },
        ], { duration: 230, easing: 'cubic-bezier(.16, 1, .3, 1)' }));
      }
    }
    if (focusField.current && !unavailable) {
      const slot = allSlots.find(key => layout[key] === focusField.current);
      if (slot) slotRefs.current.get(slot)?.focus({ preventScroll: true });
      focusField.current = null;
    }
  }, [layout, unavailable]);

  useEffect(() => {
    if (samePositions(value, toPositions(layoutRef.current))) return;
    stopDrag();
    setSelected(null);
    setHistory([]);
    updateLayout(makeLayout(value, layoutRef.current));
  }, [value, stopDrag, updateLayout]);

  useEffect(() => {
    if (!disabled) return;
    stopDrag();
    setSelected(null);
  }, [disabled, stopDrag]);

  useEffect(() => {
    mountedRef.current = true;
    const cancel = () => { stopDrag(); setSelected(null); };
    window.addEventListener('blur', cancel);
    return () => {
      mountedRef.current = false;
      window.removeEventListener('blur', cancel);
      const drag = dragRef.current;
      dragRef.current = null;
      disposeDrag(drag);
      animations.current.forEach(animation => animation.cancel());
    };
  }, [stopDrag]);

  const commit = async (next: Layout, undo = false, focus?: DecorationType) => {
    if (unavailable || pendingRef.current) return;
    const previous = layoutRef.current;
    const nextValue = toPositions(next);
    const changed = !samePositions(toPositions(previous), nextValue);
    pendingRef.current = true;
    setSelected(null);
    focusField.current = focus ?? null;
    updateLayout(next);
    if (changed) setPending(true);
    try {
      const saved = !changed || await onChange(nextValue);
      if (!mountedRef.current) return;
      if (saved) setHistory(entries => undo ? entries.slice(0, -1) : [...entries, previous]);
      else {
        focusField.current = focus ?? null;
        updateLayout(makeLayout(valueRef.current, previous));
      }
    } finally {
      pendingRef.current = false;
      if (mountedRef.current) setPending(false);
    }
  };

  const move = (field: DecorationType, destination: Slot, keyboard = false) => {
    if (unavailable || pendingRef.current) return;
    const current = layoutRef.current;
    const source = allSlots.find(slot => current[slot] === field);
    setSelected(null);
    if (!source || source === destination) return;
    void commit({ ...current, [source]: current[destination], [destination]: field }, false, keyboard ? field : undefined);
  };

  const dropTarget = (x: number, y: number): Slot | null => {
    const element = document.elementFromPoint(x, y);
    const button = element?.closest<HTMLButtonElement>('[data-decoration-slot]');
    if (button && rootRef.current?.contains(button)) return button.dataset.decorationSlot as Slot;
    if (element && poolRef.current?.contains(element)) return poolSlots.find(slot => !layoutRef.current[slot]) ?? null;
    return null;
  };

  const pointerDown = (event: PointerEvent<HTMLButtonElement>, source: Slot, field: DecorationType) => {
    if (unavailable || pendingRef.current || dragRef.current || event.button !== 0 || !event.isPrimary) return;
    const tile = tileRefs.current.get(field);
    if (!tile) return;
    animations.current.forEach(animation => animation.cancel());
    animations.current = [];
    const rect = tile.getBoundingClientRect();
    dragRef.current = {
      field, source, pointerId: event.pointerId, x: event.clientX, y: event.clientY,
      offsetX: event.clientX - rect.x, offsetY: event.clientY - rect.y, rect,
      button: event.currentTarget, tile, ghost: null,
    };
    event.currentTarget.setPointerCapture(event.pointerId);
  };

  const pointerMove = (event: PointerEvent<HTMLDivElement>) => {
    const drag = dragRef.current;
    if (!drag || event.pointerId !== drag.pointerId || unavailable) return;
    if (!drag.ghost && Math.hypot(event.clientX - drag.x, event.clientY - drag.y) < 6) return;
    event.preventDefault();
    if (!drag.ghost) {
      const ghost = drag.tile.cloneNode(true) as HTMLElement;
      ghost.className = 'decorations-tile decorations-drag-ghost';
      ghost.style.width = `${drag.rect.width}px`;
      ghost.style.height = `${drag.rect.height}px`;
      ghost.setAttribute('aria-hidden', 'true');
      document.body.append(ghost);
      document.body.classList.add('decorations-dragging');
      drag.ghost = ghost;
      setSelected(drag.field);
      setDragged(drag.field);
    }
    drag.ghost.style.transform = `translate(${event.clientX - drag.offsetX}px, ${event.clientY - drag.offsetY}px)`;
    const nextTarget = dropTarget(event.clientX, event.clientY);
    setTarget(nextTarget === drag.source ? null : nextTarget);
  };

  const pointerUp = (event: PointerEvent<HTMLDivElement>) => {
    const drag = dragRef.current;
    if (!drag || event.pointerId !== drag.pointerId) return;
    const destination = drag.ghost ? dropTarget(event.clientX, event.clientY) : null;
    const moved = Boolean(drag.ghost);
    stopDrag();
    if (!moved) return;
    event.preventDefault();
    if (destination) move(drag.field, destination);
    else setSelected(null);
  };

  const renderSlot = (slot: Slot, label: string, inPool = false) => {
    const field = layout[slot];
    const Icon = field ? icons[field] : null;
    const active = Boolean(field && selected === field);
    return (
      <button key={slot} type="button" data-decoration-slot={slot}
        ref={node => { if (node) slotRefs.current.set(slot, node); else slotRefs.current.delete(slot); }}
        disabled={unavailable} title={blocked} aria-pressed={field ? active : undefined}
        aria-label={`${label}，${field ? decorationTypeLabel(field) : '空闲位'}`}
        className={cn('decorations-slot ui-interactive', field && 'decorations-occupied', target === slot && 'decorations-drop-target')}
        onClick={event => {
          if (unavailable || pendingRef.current || performance.now() < ignoreClickUntil.current) return;
          if (selected) move(selected, slot, event.detail === 0);
          else if (field) setSelected(field);
        }}
        onPointerDown={event => { if (field) pointerDown(event, slot, field); }}
        onKeyDown={event => {
          if (!field || inPool || !['Backspace', 'Delete'].includes(event.key)) return;
          const free = poolSlots.find(key => !layoutRef.current[key]);
          if (free) { event.preventDefault(); move(field, free, true); }
        }}>
        {field && Icon ? (
          <span className={cn('decorations-tile', dragged === field && 'decorations-drag-origin')}
            ref={node => { if (node) tileRefs.current.set(field, node); else tileRefs.current.delete(field); }}>
            <Icon className="decorations-tile-icon" strokeWidth={1.75} aria-hidden="true" />
            <span className="decorations-tile-name">{decorationTypeLabel(field)}</span>
          </span>
        ) : !inPool ? <span className="decorations-empty-label" aria-hidden="true">{label}</span> : null}
      </button>
    );
  };

  const resetLayout = makeLayout(defaults);
  return (
    <div ref={rootRef} className="decorations-editor" data-selected={selected ? '' : undefined} aria-busy={pending || loading}
      onPointerMove={pointerMove} onPointerUp={pointerUp}
      onPointerCancel={() => { stopDrag(); setSelected(null); }}
      onLostPointerCapture={event => {
        if (dragRef.current?.pointerId === event.pointerId) { stopDrag(); setSelected(null); }
      }}
      onDragStart={event => event.preventDefault()}
      onKeyDown={event => { if (event.key === 'Escape') { stopDrag(); setSelected(null); } }}
      onClick={event => {
        if (event.target instanceof Element && !event.target.closest('[data-decoration-slot]')) setSelected(null);
      }}>
      <header className="decorations-editor-header">
        <div className="flex items-center gap-2">
          <AnchoredPopoverTitle className={floatingPanelTitleClassName}>页面装饰</AnchoredPopoverTitle>
          {loading && <Loader2 className="h-3.5 w-3.5 animate-spin text-text-400 motion-reduce:animate-none" aria-label="正在加载页面装饰" />}
        </div>
        <div className="flex gap-0.5">
          <IconButton label="撤销" className="h-7 w-7" disabled={unavailable || !history.length}
            onClick={() => { const previous = history[history.length - 1]; if (previous) void commit(previous, true); }}>
            <Undo2 className="h-[15px] w-[15px]" strokeWidth={1.75} />
          </IconButton>
          <IconButton label="重置位置" className="h-7 w-7" disabled={unavailable || allSlots.every(slot => layout[slot] === resetLayout[slot])}
            onClick={() => { void commit(resetLayout); }}>
            <RotateCcw className="h-[15px] w-[15px]" strokeWidth={1.75} />
          </IconButton>
        </div>
      </header>
      <div className="decorations-editor-body">
        <div className="decorations-canvas" role="group" aria-label="幻灯片装饰位置">
          <span className="decorations-canvas-ratio" aria-hidden="true">16:9</span>
          {positions.map(position => renderSlot(position.value, position.label))}
        </div>
        <h2 id={poolTitleId} className="decorations-pool-heading">未显示</h2>
        <div ref={poolRef} className="decorations-pool" role="group" aria-labelledby={poolTitleId}>
          {poolSlots.map((slot, index) => renderSlot(slot, `未显示，位置 ${index + 1}`, true))}
        </div>
      </div>
    </div>
  );
}
