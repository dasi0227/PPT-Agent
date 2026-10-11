import { useCallback, useLayoutEffect, useRef, useState, type KeyboardEvent, type PointerEvent } from 'react';

type Point = { x: number; y: number };

export function useFloatingStatusBar() {
  const barRef = useRef<HTMLElement>(null);
  const positionRef = useRef<Point | null>(null);
  const dragRef = useRef<{ pointerId: number; client: Point } | null>(null);
  const [position, setPosition] = useState<Point | null>(null);
  const [dragging, setDragging] = useState(false);

  const currentPosition = () => {
    if (positionRef.current) return positionRef.current;
    const bar = barRef.current;
    const parent = bar?.parentElement;
    if (!bar || !parent) return { x: 0, y: 0 };
    const rect = bar.getBoundingClientRect();
    const bounds = parent.getBoundingClientRect();
    return { x: rect.left - bounds.left, y: rect.top - bounds.top };
  };

  const place = useCallback((point: Point) => {
    const bar = barRef.current;
    const parent = bar?.parentElement;
    if (!bar || !parent) return;
    const toolbar = parent.querySelector<HTMLElement>(':scope > .preview-toolbar');
    const maxX = Math.max(8, parent.clientWidth - bar.offsetWidth - 8);
    const maxY = Math.max(8, parent.clientHeight - bar.offsetHeight - 8);
    const minY = Math.min((toolbar?.offsetHeight ?? 0) + 8, maxY);
    const next = {
      x: Math.max(8, Math.min(maxX, point.x)),
      y: Math.max(minY, Math.min(maxY, point.y)),
    };
    if (next.x === positionRef.current?.x && next.y === positionRef.current?.y) return;
    positionRef.current = next;
    setPosition(next);
  }, []);

  useLayoutEffect(() => {
    const bar = barRef.current;
    const parent = bar?.parentElement;
    if (!bar || !parent) return;
    const observer = new ResizeObserver(() => {
      if (positionRef.current) place(positionRef.current);
    });
    observer.observe(parent);
    observer.observe(bar);
    return () => observer.disconnect();
  }, [place]);

  const stopDragging = (event: PointerEvent<HTMLButtonElement>) => {
    if (dragRef.current?.pointerId !== event.pointerId) return;
    dragRef.current = null;
    setDragging(false);
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
  };

  return {
    barRef, position, dragging,
    handleProps: {
      onPointerDown: (event: PointerEvent<HTMLButtonElement>) => {
        if (event.button !== 0 || dragRef.current) return;
        event.preventDefault();
        place(currentPosition());
        dragRef.current = { pointerId: event.pointerId, client: { x: event.clientX, y: event.clientY } };
        event.currentTarget.setPointerCapture(event.pointerId);
        setDragging(true);
      },
      onPointerMove: (event: PointerEvent<HTMLButtonElement>) => {
        const drag = dragRef.current;
        if (!drag || drag.pointerId !== event.pointerId) return;
        const origin = currentPosition();
        place({ x: origin.x + event.clientX - drag.client.x, y: origin.y + event.clientY - drag.client.y });
        drag.client = { x: event.clientX, y: event.clientY };
      },
      onPointerUp: stopDragging,
      onPointerCancel: stopDragging,
      onLostPointerCapture: stopDragging,
      onKeyDown: (event: KeyboardEvent<HTMLButtonElement>) => {
        const direction: Record<string, Point> = {
          ArrowLeft: { x: -1, y: 0 }, ArrowRight: { x: 1, y: 0 },
          ArrowUp: { x: 0, y: -1 }, ArrowDown: { x: 0, y: 1 },
        };
        const delta = direction[event.key];
        if (!delta) return;
        event.preventDefault();
        const origin = currentPosition();
        const step = event.shiftKey ? 24 : 8;
        place({ x: origin.x + delta.x * step, y: origin.y + delta.y * step });
      },
    },
  };
}
