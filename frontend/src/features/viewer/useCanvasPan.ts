import { useCallback, useLayoutEffect, useRef, useState, type PointerEvent, type RefObject } from 'react';

type Point = { x: number; y: number };
type Bounds = { minX: number; maxX: number; minY: number; maxY: number };
type StageLayout = { width: number; height: number; top: number };
const ORIGIN: Point = { x: 0, y: 0 };
const NO_BOUNDS: Bounds = { minX: 0, maxX: 0, minY: 0, maxY: 0 };

function clampPosition(position: Point, bounds: Bounds): Point {
  return {
    x: Math.max(bounds.minX, Math.min(bounds.maxX, position.x)),
    y: Math.max(bounds.minY, Math.min(bounds.maxY, position.y)),
  };
}

export function useCanvasPan({
  viewportRef, enabled, layoutEnabled, selectionActive, zoom, pageKey,
}: {
  viewportRef: RefObject<HTMLDivElement | null>;
  enabled: boolean;
  layoutEnabled: boolean;
  selectionActive: boolean;
  zoom: number;
  pageKey: string;
}) {
  const stageRef = useRef<HTMLDivElement>(null);
  const [position, setPosition] = useState<Point>(ORIGIN);
  const [bounds, setBounds] = useState<Bounds>(NO_BOUNDS);
  const [stageLayout, setStageLayout] = useState<StageLayout>();
  const [dragging, setDragging] = useState(false);
  const dragRef = useRef<{ pointerId: number; client: Point; origin: Point } | null>(null);
  const canPan = enabled && !selectionActive && (bounds.minX < bounds.maxX || bounds.minY < bounds.maxY);

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
    if (!layoutEnabled || !viewport || !stage) {
      setBounds(NO_BOUNDS);
      setStageLayout(undefined);
      return;
    }
    const visualViewport = window.visualViewport;
    const measure = () => {
      const padding = getComputedStyle(viewport);
      const paddingTop = parseFloat(padding.paddingTop);
      const paddingBottom = parseFloat(padding.paddingBottom);
      const availableWidth = Math.max(0, viewport.clientWidth - parseFloat(padding.paddingLeft) - parseFloat(padding.paddingRight));
      const availableHeight = Math.max(0, viewport.clientHeight - paddingTop - paddingBottom);
      const width = Math.min(availableWidth, availableHeight * 16 / 9, 1024);
      const height = width * 9 / 16;
      const pageCenter = (visualViewport?.offsetTop ?? 0) + (visualViewport?.height ?? window.innerHeight) / 2;
      // Aim at the whole page's vertical midpoint, constrained by the controls above and below.
      const top = Math.max(paddingTop + height / 2, Math.min(
        viewport.clientHeight - paddingBottom - height / 2,
        pageCenter - viewport.getBoundingClientRect().top,
      ));
      setStageLayout((value) => value?.width === width && value.height === height && value.top === top
        ? value : { width, height, top });
      const scaledWidth = width * zoom;
      const scaledHeight = height * zoom;
      const next: Bounds = enabled ? {
        minX: Math.min(0, (viewport.clientWidth - scaledWidth) / 2),
        maxX: Math.max(0, (scaledWidth - viewport.clientWidth) / 2),
        minY: Math.min(0, viewport.clientHeight - top - scaledHeight / 2),
        maxY: Math.max(0, scaledHeight / 2 - top),
      } : NO_BOUNDS;
      setBounds(next);
      setPosition((value) => clampPosition(value, next));
      stopDragging();
    };
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(viewport);
    observer.observe(document.documentElement);
    window.addEventListener('resize', measure);
    visualViewport?.addEventListener('resize', measure);
    visualViewport?.addEventListener('scroll', measure);
    return () => {
      observer.disconnect();
      window.removeEventListener('resize', measure);
      visualViewport?.removeEventListener('resize', measure);
      visualViewport?.removeEventListener('scroll', measure);
    };
  }, [enabled, layoutEnabled, zoom, pageKey, viewportRef, stopDragging]);

  useLayoutEffect(() => {
    if (!canPan) stopDragging();
  }, [canPan, stopDragging]);

  return {
    stageRef,
    stageLayout,
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
