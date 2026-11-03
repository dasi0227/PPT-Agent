import { useCallback, useLayoutEffect, useRef, useState } from 'react';
import type { KeyboardEvent, MouseEvent, PointerEvent, TouchEvent, WheelEvent } from 'react';

type ScrollOptions = {
  threadId: string | null;
  generation?: number;
  latestTurnId: string | null;
  newInput: boolean;
  pendingInput: boolean;
  historyLoading: boolean;
  groupPages?: Readonly<Record<string, { status: string }>>;
};
type Position = 'bottom' | 'turn' | 'manual';
type Metrics = { top: number; width: number; height: number; contentHeight: number };
type ViewAnchor = { node: HTMLElement; key: string; offset: number };

const topPadding = 12;
const returnDistance = 80;
const readMetrics = (node: HTMLDivElement): Metrics => ({
  top: node.scrollTop, width: node.clientWidth, height: node.clientHeight, contentHeight: node.scrollHeight,
});
const bottom = (metrics: Metrics) => Math.max(0, metrics.contentHeight - metrics.height);

function captureAnchor(container: HTMLDivElement, previous: ViewAnchor | null, scan = false): ViewAnchor | null {
  const bounds = container.getBoundingClientRect();
  const top = bounds.top;
  for (const inset of [topPadding, topPadding + 12, topPadding + 36]) {
    const hit = container.ownerDocument.elementFromPoint?.(bounds.left + bounds.width / 2, top + inset);
    const node = hit?.closest<HTMLElement>('[data-timeline-key]');
    if (node && container.contains(node)) {
      return { node, key: node.dataset.timelineKey!, offset: node.getBoundingClientRect().top - top };
    }
  }
  if (previous?.node.isConnected) {
    const rect = previous.node.getBoundingClientRect();
    if (rect.top <= top + topPadding && rect.bottom > top + topPadding) {
      return { ...previous, offset: rect.top - top };
    }
  }
  // Only an explicit pagination request may scan the loaded rows as a fallback.
  if (!scan) return null;
  let anchor: ViewAnchor | null = null;
  for (const node of container.querySelectorAll<HTMLElement>('[data-timeline-key]')) {
    if (anchor && !anchor.node.contains(node)) break;
    const rect = node.getBoundingClientRect();
    if (rect.bottom <= top) continue;
    anchor = { node, key: node.dataset.timelineKey!, offset: rect.top - top };
  }
  return anchor;
}

export function useTimelineScroll(options: ScrollOptions) {
  const containerRef = useRef<HTMLDivElement>(null);
  const contentRef = useRef<HTMLDivElement>(null);
  const latestTurnRef = useRef<HTMLDivElement>(null);
  const committed = useRef<ScrollOptions | null>(null);
  const previous = useRef<{ scope: string; turnId: string | null } | null>(null);
  const position = useRef<Position>('bottom');
  const metricsRef = useRef<Metrics | null>(null);
  const viewAnchor = useRef<ViewAnchor | null>(null);
  const capturePending = useRef(false);
  const pendingPage = useRef<{ groupId?: string } | null>(null);
  const frame = useRef<number | null>(null);
  const smoothTarget = useRef<number | null>(null);
  const allowFollowing = useRef(false);
  const touchY = useRef<number | null>(null);
  const [anchoredTurnId, setAnchoredTurnId] = useState<string | null>(null);
  const [showReturn, setShowReturn] = useState(false);

  const stopSmooth = useCallback(() => {
    const container = containerRef.current;
    if (smoothTarget.current === null || !container) return;
    smoothTarget.current = null;
    // An immediate scroll to the current position cancels native smooth scrolling.
    container.scrollTo?.({ top: container.scrollTop, behavior: 'auto' });
    metricsRef.current = readMetrics(container);
  }, []);

  const flush = useCallback(() => {
    frame.current = null;
    const container = containerRef.current;
    const current = committed.current;
    if (!container || !current) return;
    const metrics = readMetrics(container);
    const write = (top: number) => {
      const target = Math.max(0, Math.min(bottom(metrics), top));
      if (Math.abs(container.scrollTop - target) > 1) container.scrollTop = target;
      metrics.top = container.scrollTop;
      metricsRef.current = metrics;
      return target;
    };

    // While the user-requested animation runs, content updates must not replace it.
    if (smoothTarget.current !== null) {
      if (Math.abs(metrics.top - Math.min(smoothTarget.current, bottom(metrics))) > 1) {
        metricsRef.current = metrics;
        return;
      }
      smoothTarget.current = null;
    }

    if (position.current === 'bottom') {
      write(bottom(metrics));
      setShowReturn(false);
      return;
    }
    if (position.current === 'turn' && latestTurnRef.current) {
      write(metrics.top + latestTurnRef.current.getBoundingClientRect().top
        - container.getBoundingClientRect().top - topPadding);
    } else if (position.current === 'manual') {
      const page = pendingPage.current;
      const loading = page && (page.groupId ? current.groupPages?.[page.groupId]?.status === 'loading' : current.historyLoading);
      if (!loading) {
        pendingPage.current = null;
        if (capturePending.current) {
          viewAnchor.current = captureAnchor(container, viewAnchor.current);
          capturePending.current = false;
        }
        const anchor = viewAnchor.current;
        if (anchor) {
          const node = anchor.node.isConnected ? anchor.node
            : Array.from(container.querySelectorAll<HTMLElement>('[data-timeline-key]')).find(value => value.dataset.timelineKey === anchor.key);
          if (node) {
            const target = metrics.top + node.getBoundingClientRect().top - container.getBoundingClientRect().top - anchor.offset;
            const clamped = write(target);
            anchor.node = node;
            anchor.offset += target - clamped;
          } else viewAnchor.current = null;
        }
        if (!viewAnchor.current) viewAnchor.current = captureAnchor(container, null);
        if (allowFollowing.current && bottom(metrics) - metrics.top <= returnDistance) {
          position.current = 'bottom';
          viewAnchor.current = null;
          write(bottom(metrics));
        }
      }
    }
    metricsRef.current = metrics;
    setShowReturn(position.current !== 'bottom' && bottom(metrics) - metrics.top > returnDistance);
  }, []);

  const schedule = useCallback(() => {
    if (frame.current === null) frame.current = window.requestAnimationFrame(flush);
  }, [flush]);

  useLayoutEffect(() => {
    committed.current = options;
    const scope = `${options.threadId}:${options.generation ?? 0}`;
    const prior = previous.current;
    if (!prior || prior.scope !== scope) {
      stopSmooth();
      position.current = options.pendingInput ? 'turn' : 'bottom';
      viewAnchor.current = null;
      capturePending.current = false;
      pendingPage.current = null;
      allowFollowing.current = false;
      metricsRef.current = null;
      setAnchoredTurnId(options.pendingInput ? options.latestTurnId : null);
      setShowReturn(false);
    } else if (options.newInput && options.latestTurnId && options.latestTurnId !== prior.turnId) {
      stopSmooth();
      position.current = 'turn';
      viewAnchor.current = null;
      capturePending.current = false;
      pendingPage.current = null;
      allowFollowing.current = false;
      setAnchoredTurnId(options.latestTurnId);
    }
    previous.current = { scope, turnId: options.latestTurnId };
    schedule();
  });

  useLayoutEffect(() => {
    const container = containerRef.current;
    const content = contentRef.current;
    if (!container) return;
    const observer = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(schedule);
    observer?.observe(container);
    if (content) observer?.observe(content);
    const finishSmooth = () => {
      if (smoothTarget.current === null) return;
      smoothTarget.current = null;
      schedule();
    };
    container.addEventListener('scrollend', finishSmooth);
    return () => {
      observer?.disconnect();
      container.removeEventListener('scrollend', finishSmooth);
      if (frame.current !== null) window.cancelAnimationFrame(frame.current);
      frame.current = null;
      committed.current = null;
    };
  }, [schedule]);

  const interrupt = useCallback((down: boolean) => {
    stopSmooth();
    position.current = 'manual';
    allowFollowing.current = down;
    capturePending.current = true;
    pendingPage.current = null;
    schedule();
  }, [schedule, stopSmooth]);

  const handleScroll = useCallback(() => {
    const container = containerRef.current;
    if (!container) return;
    const metrics = readMetrics(container);
    const last = metricsRef.current;
    metricsRef.current = metrics;
    const resized = last && (last.width !== metrics.width || last.height !== metrics.height || last.contentHeight !== metrics.contentHeight);
    if (last && !resized && Math.abs(metrics.top - last.top) <= 1 && smoothTarget.current === null) return;
    if (last && Math.abs(metrics.top - last.top) > 1 && !resized && smoothTarget.current === null) {
      if (metrics.top < last.top || position.current !== 'bottom') {
        position.current = 'manual';
        allowFollowing.current = metrics.top > last.top;
        capturePending.current = true;
        pendingPage.current = null;
      }
    }
    schedule();
  }, [schedule]);

  const scrollToLatest = useCallback(() => {
    const container = containerRef.current;
    if (!container) return;
    stopSmooth();
    position.current = 'bottom';
    viewAnchor.current = null;
    capturePending.current = false;
    pendingPage.current = null;
    allowFollowing.current = false;
    setShowReturn(false);
    const target = bottom(readMetrics(container));
    const reducedMotion = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches;
    if (!reducedMotion && container.scrollTo && Math.abs(container.scrollTop - target) > 1) {
      smoothTarget.current = target;
      container.scrollTo({ top: target, behavior: 'smooth' });
    }
    schedule();
  }, [schedule, stopSmooth]);

  const keepPageAnchor = useCallback((groupId?: string) => {
    const container = containerRef.current;
    if (!container) return;
    stopSmooth();
    position.current = 'manual';
    allowFollowing.current = false;
    viewAnchor.current = captureAnchor(container, viewAnchor.current, true);
    capturePending.current = false;
    pendingPage.current = { groupId };
  }, [stopSmooth]);

  const holdRow = useCallback((target: Element) => {
    const container = containerRef.current;
    const node = target.closest<HTMLElement>('[data-timeline-key]');
    if (!container || !node || !container.contains(node)) return;
    stopSmooth();
    position.current = 'manual';
    allowFollowing.current = false;
    viewAnchor.current = {
      node, key: node.dataset.timelineKey!,
      offset: node.getBoundingClientRect().top - container.getBoundingClientRect().top,
    };
    capturePending.current = false;
    pendingPage.current = null;
    metricsRef.current = readMetrics(container);
    schedule();
  }, [schedule, stopSmooth]);

  const handleClickCapture = useCallback((event: MouseEvent<HTMLDivElement>) => {
    const trigger = (event.target as Element).closest<HTMLElement>('[aria-expanded]');
    if (trigger && !trigger.matches(':disabled, [aria-disabled="true"]')) holdRow(trigger);
  }, [holdRow]);

  const handleWheel = useCallback((event: WheelEvent<HTMLDivElement>) => {
    if (event.deltaY !== 0 && !event.ctrlKey) interrupt(event.deltaY > 0);
  }, [interrupt]);
  const handlePointerDown = useCallback((event: PointerEvent<HTMLDivElement>) => {
    if (event.button > 0) return;
    const target = event.target as Element;
    if (target.closest('.prose, .agent-compact-markdown')) {
      holdRow(target);
      return;
    }
    if (event.target === event.currentTarget && smoothTarget.current !== null) interrupt(false);
  }, [holdRow, interrupt]);
  const handleTouchStart = useCallback((event: TouchEvent<HTMLDivElement>) => {
    touchY.current = event.touches[0]?.clientY ?? null;
  }, []);
  const handleTouchMove = useCallback((event: TouchEvent<HTMLDivElement>) => {
    const nextY = event.touches[0]?.clientY;
    if (nextY === undefined || touchY.current === null) return;
    const delta = touchY.current - nextY;
    touchY.current = nextY;
    if (delta !== 0) interrupt(delta > 0);
  }, [interrupt]);
  const handleKeyDown = useCallback((event: KeyboardEvent<HTMLDivElement>) => {
    const target = event.target as HTMLElement;
    if (target.closest('input, textarea, select, [contenteditable="true"], [role="textbox"]')) return;
    const trigger = target.closest<HTMLElement>('[aria-expanded]');
    if (trigger && !trigger.matches(':disabled, [aria-disabled="true"]') && (event.key === 'Enter' || event.key === ' ')) {
      holdRow(trigger);
      return;
    }
    if (event.defaultPrevented) return;
    if (['ArrowUp', 'PageUp', 'Home'].includes(event.key)) interrupt(false);
    else if (['ArrowDown', 'PageDown', 'End'].includes(event.key)) interrupt(true);
    else if (event.key === ' ' && !target.closest('button, a')) interrupt(!event.shiftKey);
  }, [holdRow, interrupt]);

  return {
    containerRef, contentRef, latestTurnRef, anchoredTurnId, showReturn, scrollToLatest, keepPageAnchor,
    handleScroll, handleWheel, handlePointerDown, handleClickCapture, handleTouchStart, handleTouchMove, handleKeyDown,
  };
}
