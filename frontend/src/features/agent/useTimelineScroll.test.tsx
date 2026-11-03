import { useLayoutEffect } from 'react';
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { useTimelineScroll } from './useTimelineScroll';

type Options = Parameters<typeof useTimelineScroll>[0];
let geometry: { top: number; height: number; contentHeight: number; turnTop: number; rowTops: number[] };
let frames: Map<number, FrameRequestCallback>;
let nextFrame: number;
let resizeCallbacks: Set<() => void>;
let observed: Element[];
let writes: number[];
let scrollTo: ReturnType<typeof vi.fn>;
const elementFromPointDescriptor = Object.getOwnPropertyDescriptor(document, 'elementFromPoint');

const rect = (top: number, height: number) => ({ top, bottom: top + height, height, left: 0, right: 350, width: 350, x: 0, y: top, toJSON() {} });

function Harness({ options = {} }: { options?: Partial<Options> }) {
  const scroll = useTimelineScroll({
    threadId: 't1', generation: 0, latestTurnId: 'turn1', newInput: false, pendingInput: false,
    historyLoading: false, ...options,
  });
  useLayoutEffect(() => {
    const container = scroll.containerRef.current!;
    Object.defineProperties(container, {
      scrollTop: { configurable: true, get: () => geometry.top, set: (value: number) => {
        geometry.top = Math.max(0, Math.min(geometry.contentHeight - geometry.height, value));
        writes.push(geometry.top);
      } },
      clientWidth: { configurable: true, get: () => 350 },
      clientHeight: { configurable: true, get: () => geometry.height },
      scrollHeight: { configurable: true, get: () => geometry.contentHeight },
    });
    container.getBoundingClientRect = () => rect(0, geometry.height);
    container.scrollTo = scrollTo;
    scroll.latestTurnRef.current!.getBoundingClientRect = () => rect(geometry.turnTop - geometry.top, 300);
  }, [scroll.containerRef, scroll.latestTurnRef]);
  return <>
    <div ref={scroll.containerRef} data-testid="viewport" onScroll={scroll.handleScroll} onWheel={scroll.handleWheel} onPointerDown={scroll.handlePointerDown}
      onTouchStart={scroll.handleTouchStart} onTouchMove={scroll.handleTouchMove} onKeyDown={scroll.handleKeyDown}>
      <div ref={scroll.contentRef} data-testid="content">
        {geometry.rowTops.map((_, index) => <div key={index} data-timeline-key={`row${index}`} ref={node => {
          if (node) node.getBoundingClientRect = () => rect(geometry.rowTops[index] - geometry.top, 500);
        }} />)}
        <div ref={scroll.latestTurnRef} data-testid="turn" data-anchored={scroll.anchoredTurnId} />
        <textarea aria-label="input" />
      </div>
    </div>
    <button onClick={() => scroll.keepPageAnchor()}>history</button>
    <button onClick={() => scroll.keepPageAnchor('g1')}>group</button>
    {scroll.showReturn && <button onClick={scroll.scrollToLatest}>latest</button>}
  </>;
}

function flushFrames() {
  act(() => {
    const pending = [...frames.values()];
    frames.clear();
    pending.forEach(callback => callback(0));
  });
}
function settle() {
  for (let count = 0; frames.size && count < 5; count += 1) flushFrames();
  expect(frames.size).toBe(0);
}
function resize() {
  act(() => resizeCallbacks.forEach(callback => callback()));
}
function userScroll(top: number) {
  geometry.top = top;
  fireEvent.scroll(screen.getByTestId('viewport'));
}

beforeEach(() => {
  geometry = { top: 0, height: 400, contentHeight: 1000, turnTop: 500, rowTops: [0, 500, 1000] };
  frames = new Map();
  nextFrame = 0;
  resizeCallbacks = new Set();
  observed = [];
  writes = [];
  scrollTo = vi.fn((options: ScrollToOptions) => {
    if (options.behavior !== 'smooth') geometry.top = options.top ?? geometry.top;
  });
  vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => {
    frames.set(++nextFrame, callback);
    return nextFrame;
  });
  vi.stubGlobal('cancelAnimationFrame', (id: number) => frames.delete(id));
  vi.stubGlobal('matchMedia', () => ({ matches: false }));
  Object.defineProperty(document, 'elementFromPoint', { configurable: true, value: (_x: number, y: number) => {
    const index = geometry.rowTops.findIndex(top => top <= geometry.top + y && top + 500 > geometry.top + y);
    return index < 0 ? null : document.querySelector(`[data-timeline-key="row${index}"]`);
  } });
  vi.stubGlobal('ResizeObserver', class {
    callback: () => void;
    constructor(callback: () => void) { this.callback = callback; resizeCallbacks.add(callback); }
    observe(node: Element) { observed.push(node); }
    disconnect() { resizeCallbacks.delete(this.callback); }
  });
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  if (elementFromPointDescriptor) Object.defineProperty(document, 'elementFromPoint', elementFromPointDescriptor);
  else Reflect.deleteProperty(document, 'elementFromPoint');
});

it('coalesces content, progress and size updates into one immediate follow without treating its scroll as user input', () => {
  const view = render(<Harness />);
  settle();
  expect(geometry.top).toBe(600);
  expect(observed).toEqual([screen.getByTestId('viewport'), screen.getByTestId('content')]);
  writes.length = 0;
  geometry.contentHeight = 1200;
  view.rerender(<Harness />);
  geometry.contentHeight = 1400;
  view.rerender(<Harness />);
  resize();
  expect(frames.size).toBe(1);
  settle();
  expect(writes).toEqual([1000]);
  expect(scrollTo).not.toHaveBeenCalled();
  fireEvent.scroll(screen.getByTestId('viewport'));
  geometry.contentHeight = 1500;
  resize();
  settle();
  expect(geometry.top).toBe(1100);
});

it('stops following on a small upward wheel before a queued update, and lets the return button resume following', () => {
  const view = render(<Harness />);
  settle();
  geometry.contentHeight = 1020;
  view.rerender(<Harness />);
  const scan = vi.spyOn(screen.getByTestId('viewport'), 'querySelectorAll');
  fireEvent.wheel(screen.getByTestId('viewport'), { deltaY: -20 });
  userScroll(580);
  settle();
  expect(geometry.top).toBe(580);
  expect(scan).not.toHaveBeenCalled();
  geometry.contentHeight = 1400;
  resize();
  settle();
  expect(geometry.top).toBe(580);
  fireEvent.click(screen.getByRole('button', { name: 'latest' }));
  expect(scrollTo).toHaveBeenLastCalledWith({ top: 1000, behavior: 'smooth' });
  userScroll(700);
  geometry.contentHeight = 1600;
  view.rerender(<Harness />);
  resize();
  settle();
  expect(geometry.top).toBe(700);
  userScroll(1000);
  settle();
  expect(geometry.top).toBe(1200);
  expect(screen.queryByRole('button', { name: 'latest' })).not.toBeInTheDocument();
});

it('lets scrolling gestures interrupt a smooth return while composer keys and pointer events on child content keep it running', () => {
  render(<Harness />);
  settle();
  const viewport = screen.getByTestId('viewport');
  fireEvent.wheel(viewport, { deltaY: -300 });
  userScroll(200);
  settle();
  fireEvent.click(screen.getByRole('button', { name: 'latest' }));
  fireEvent.keyDown(screen.getByLabelText('input'), { key: 'ArrowUp' });
  fireEvent.pointerDown(screen.getByTestId('content'));
  expect(scrollTo).toHaveBeenCalledTimes(1);
  fireEvent.pointerDown(viewport);
  expect(scrollTo).toHaveBeenLastCalledWith({ top: 200, behavior: 'auto' });
  settle();
  fireEvent.click(screen.getByRole('button', { name: 'latest' }));
  fireEvent.wheel(viewport, { deltaY: -20 });
  expect(scrollTo).toHaveBeenLastCalledWith({ top: 200, behavior: 'auto' });
  settle();
  fireEvent.click(screen.getByRole('button', { name: 'latest' }));
  fireEvent.touchStart(viewport, { touches: [{ clientY: 100 }] });
  fireEvent.touchMove(viewport, { touches: [{ clientY: 130 }] });
  expect(scrollTo).toHaveBeenLastCalledWith({ top: 200, behavior: 'auto' });
  settle();
  fireEvent.click(screen.getByRole('button', { name: 'latest' }));
  fireEvent.keyDown(viewport, { key: 'PageUp' });
  settle();
  geometry.contentHeight = 1300;
  resize();
  settle();
  expect(geometry.top).toBe(200);
  fireEvent.keyDown(viewport, { key: 'End' });
  userScroll(890);
  settle();
  expect(geometry.top).toBe(900);
});

it('keeps a new input at the top through content and viewport resizing, then resets on a scene or thread switch', () => {
  const view = render(<Harness />);
  settle();
  geometry.contentHeight = 1700;
  geometry.turnTop = 1000;
  view.rerender(<Harness options={{ latestTurnId: 'turn2', newInput: true }} />);
  settle();
  expect(geometry.top).toBe(988);
  expect(screen.getByTestId('turn')).toHaveAttribute('data-anchored', 'turn2');
  geometry.turnTop = 1100;
  geometry.contentHeight = 2000;
  geometry.height = 500;
  resize();
  settle();
  expect(geometry.top).toBe(1088);
  fireEvent.wheel(screen.getByTestId('viewport'), { deltaY: -100 });
  userScroll(900);
  settle();
  resize();
  settle();
  expect(geometry.top).toBe(900);
  view.rerender(<Harness options={{ latestTurnId: 'turn2', generation: 1 }} />);
  settle();
  expect(geometry.top).toBe(1500);
  expect(screen.getByTestId('turn')).not.toHaveAttribute('data-anchored');
  geometry.contentHeight = 1000;
  view.rerender(<Harness options={{ threadId: 't2' }} />);
  settle();
  expect(geometry.top).toBe(500);
});

it.each(['history', 'group'])('preserves the visible anchor while %s pagination and later image expansion add content above it', page => {
  const view = render(<Harness />);
  settle();
  fireEvent.wheel(screen.getByTestId('viewport'), { deltaY: -300 });
  userScroll(550);
  settle();
  fireEvent.click(screen.getByRole('button', { name: page }));
  view.rerender(<Harness options={{ historyLoading: page === 'history', groupPages: { g1: { status: 'loading' } } }} />);
  geometry.rowTops = geometry.rowTops.map(top => top + 300);
  geometry.turnTop += 300;
  geometry.contentHeight += 300;
  resize();
  settle();
  expect(geometry.top).toBe(550);
  view.rerender(<Harness options={{ groupPages: { g1: { status: 'ready' } } }} />);
  settle();
  expect(geometry.top).toBe(850);
  geometry.rowTops = geometry.rowTops.map(top => top + 120);
  geometry.contentHeight += 120;
  resize();
  settle();
  expect(geometry.top).toBe(970);
  fireEvent.wheel(screen.getByTestId('viewport'), { deltaY: -50 });
  userScroll(920);
  settle();
  expect(geometry.top).toBe(920);
});

it('cancels pending work and observers on unmount', () => {
  const view = render(<Harness />);
  expect(frames.size).toBe(1);
  view.unmount();
  expect(frames.size).toBe(0);
  expect(resizeCallbacks.size).toBe(0);
});
