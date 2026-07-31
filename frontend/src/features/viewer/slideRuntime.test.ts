import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import { JSDOM } from 'jsdom';

function createRuntime() {
  const html = readFileSync(
    resolve(process.cwd(), 'public/slide-runtime/index.html'),
    'utf8',
  );
  return new JSDOM(html, {
    runScripts: 'dangerously',
    url: 'http://localhost/slide-runtime/index.html',
  });
}

describe('slide runtime', () => {
  it('renders validated HTML with srcdoc and switches without rebuilding frames', () => {
    const dom = createRuntime();
    const { window } = dom;
    const slides = [
      { id: 's1', html: '<!doctype html><title>one</title>' },
      { id: 's2', html: '<!doctype html><title>two</title>' },
    ];

    window.dispatchEvent(new window.MessageEvent('message', {
      source: window as unknown as Window,
      data: { type: 'updateDeck', slides, index: 0 },
    }));

    const initialFrames = Array.from(
      window.document.querySelectorAll('[data-slide-frame]'),
    ) as HTMLIFrameElement[];

    expect(initialFrames).toHaveLength(2);
    expect(initialFrames.map((frame) => frame.getAttribute('srcdoc'))).toEqual(slides.map((slide) => slide.html));
    expect(initialFrames.every((frame) => frame.getAttribute('sandbox') === 'allow-scripts')).toBe(true);
    expect(initialFrames[0]?.dataset.active).toBe('true');
    expect(initialFrames[1]?.dataset.active).toBe('false');

    window.dispatchEvent(new window.MessageEvent('message', {
      source: window as unknown as Window,
      data: { type: 'gotoSlide', index: 1 },
    }));

    const afterGotoFrames = Array.from(
      window.document.querySelectorAll('[data-slide-frame]'),
    ) as HTMLIFrameElement[];
    expect(afterGotoFrames[0]).toBe(initialFrames[0]);
    expect(afterGotoFrames[1]).toBe(initialFrames[1]);
    expect(afterGotoFrames[0]?.dataset.active).toBe('false');
    expect(afterGotoFrames[1]?.dataset.active).toBe('true');
    dom.window.close();
  });

  it('ignores undeclared payloads and messages not sent by the parent', () => {
    const dom = createRuntime();
    const { window } = dom;
    const validDeck = {
      type: 'updateDeck',
      slides: [{ id: 's1', html: '<h1>safe</h1>' }],
      index: 0,
    };

    window.dispatchEvent(new window.MessageEvent('message', {
      source: {} as Window,
      data: validDeck,
    }));
    window.dispatchEvent(new window.MessageEvent('message', {
      source: window as unknown as Window,
      data: { type: 'executeScript', callback: 'alert(1)' },
    }));

    expect(window.document.querySelectorAll('[data-slide-frame]')).toHaveLength(0);
    dom.window.close();
  });
});
