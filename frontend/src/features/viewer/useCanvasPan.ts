import { useCallback, useLayoutEffect, useRef, useState, type PointerEvent, type RefObject } from 'react';

type Point = { x: number; y: number };
const ORIGIN: Point = { x: 0, y: 0 };

function clampPosition(position: Point, bounds: Point): Point {
  return {
    x: Math.max(-bounds.x, Math.min(bounds.x, position.x)),
    y: Math.max(-bounds.y, Math.min(bounds.y, position.y)),
  };
}

export function useCanvasPan({
  viewportRef, enabled, selectionActive, zoom, pageKey,
}: {
  viewportRef: RefObject<HTMLDivElement | null>;
  enabled: boolean;
  selectionActive: boolean;
  zoom: number;
  pageKey: string;
}) {
  const stageRef = useRef<HTMLDivElement>(null);
  const [position, setPosition] = useState<Point>(ORIGIN);
  const [bounds, setBounds] = useState<Point>(ORIGIN);
  const [dragging, setDragging] = useState(false);
  const dragRef = useRef<{ pointerId: number; client: Point; origin: Point } | null>(null);
  const canPan = enabled && !selectionActive && (bounds.x > 0 || bounds.y > 0);

  const stopDragging = useCallback(() => {
    const drag = dragRef.current;
    dragRef.current = null;
    setDragging(false);
    const viewport = viewportRef.current;
    if (drag && viewport?.hasPointerCapture(drag.pointerId)) {
      viewport.releasePointerCapture(drag.pointerId);
    }
  }, [viewportRef]);

  useLayoutEffect(() => {
    setPosition(ORIGIN);
  }, [enabled, pageKey]);

  useLayoutEffect(() => {
    const viewport = viewportRef.current;
    const stage = stageRef.current;
    if (!enabled || !viewport || !stage) {
      setBounds(ORIGIN);
      return;
    }
    const measure = () => {
      // Runtime fits a centered 16:9 slide inside the stage; exclude its letterboxing.
      const width = Math.min(stage.clientWidth, stage.clientHeight * 16 / 9) * zoom;
      const next = {
        x: Math.max(0, (width - viewport.clientWidth) / 2),
        y: Math.max(0, (width * 9 / 16 - viewport.clientHeight) / 2),
      };
      setBounds(next);
      setPosition((value) => clampPosition(value, next));
      stopDragging();
    };
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(viewport);
    observer.observe(stage);
    return () => observer.disconnect();
  }, [enabled, zoom, pageKey, viewportRef, stopDragging]);

  useLayoutEffect(() => {
    if (!canPan) stopDragging();
  }, [canPan, stopDragging]);

  return {
    stageRef,
    position,
    canPan,
    dragging,
    pointerHandlers: {
      onPointerDown(event: PointerEvent<HTMLDivElement>) {
        if (!canPan || event.button !== 0 || !event.isPrimary) return;
        if ((event.target as HTMLElement).closest('button, a, input, textarea, select, summary, [role="button"]')) return;
        event.preventDefault();
        event.currentTarget.setPointerCapture(event.pointerId);
        dragRef.current = {
          pointerId: event.pointerId,
          client: { x: event.clientX, y: event.clientY },
          origin: position,
        };
        setDragging(true);
      },
      onPointerMove(event: PointerEvent<HTMLDivElement>) {
        const drag = dragRef.current;
        if (!drag || drag.pointerId !== event.pointerId) return;
        setPosition(clampPosition({
          x: drag.origin.x + event.clientX - drag.client.x,
          y: drag.origin.y + event.clientY - drag.client.y,
        }, bounds));
      },
      onPointerUp: stopDragging,
      onPointerCancel: stopDragging,
      onLostPointerCapture: stopDragging,
    },
  };
}
