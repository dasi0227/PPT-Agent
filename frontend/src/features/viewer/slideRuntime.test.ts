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
  const frame = (id: string, ordinal: number, visible = true) => ({
    slide_id: id, ordinal, total: 2, role: ordinal === 1 ? 'cover' : 'content',
    canvas: { width: 1920, height: 1080, aspect_ratio: '16:9' },
    theme_id: 'swiss-modern',
    section: { id: 'sec_1', title: '正文', index: 1 }, numbering: { visible, format: 'number' },
    deck_title: 'Deck',
    chrome: [
      { type: 'page_number', placement: 'bottom-right', style: 'tiny muted mono counter' },
      { type: 'section_marker', placement: 'top-left', style: 'muted label' },
      { type: 'deck_title', placement: 'top-right', style: 'muted label' },
    ],
  });

  it('renders validated HTML with srcdoc and switches without rebuilding frames', () => {
    const dom = createRuntime();
    const { window } = dom;
    const slides = [
      { id: 's1', html: '<!doctype html><title>one</title>', frame: frame('s1', 1, false) },
      { id: 's2', html: '<!doctype html><title>two</title>', frame: frame('s2', 2) },
    ];

    window.dispatchEvent(new window.MessageEvent('message', {
      source: window as unknown as Window,
      data: { type: 'updateDeck', slides, index: 0 },
    }));

    const initialFrames = Array.from(
      window.document.querySelectorAll('[data-slide-frame]'),
    ) as HTMLElement[];

    expect(initialFrames).toHaveLength(2);
    const srcdocs = initialFrames.map((container) => container.querySelector('iframe')?.getAttribute('srcdoc') ?? '');
    expect(srcdocs.every((srcdoc) => srcdoc.includes('id="base-link"'))).toBe(true);
    expect(srcdocs.every((srcdoc) => srcdoc.includes('/api/v1/themes/swiss-modern/css'))).toBe(true);
    expect(srcdocs.every((srcdoc) => srcdoc.includes('/slide-runtime/selection-bridge.js'))).toBe(true);
    expect(initialFrames[0]?.querySelector('.runtime-canvas')).not.toBeNull();
    expect(initialFrames.every((container) => container.querySelector('iframe')?.getAttribute('sandbox') === 'allow-scripts')).toBe(true);
    expect(initialFrames[0]?.querySelector('[data-runtime-page-number]')).toBeNull();
    expect(initialFrames[1]?.querySelector('[data-runtime-page-number]')?.textContent).toBe('2');
    expect(initialFrames[1]?.querySelector('[data-runtime-chrome="section_marker"]')?.textContent).toBe('正文');
    expect(initialFrames[1]?.querySelector('[data-runtime-chrome="deck_title"]')?.textContent).toBe('Deck');
    expect(initialFrames[0]?.dataset.active).toBe('true');
    expect(initialFrames[1]?.dataset.active).toBe('false');

    window.dispatchEvent(new window.MessageEvent('message', {
      source: window as unknown as Window,
      data: { type: 'gotoSlide', index: 1 },
    }));

    const afterGotoFrames = Array.from(
      window.document.querySelectorAll('[data-slide-frame]'),
    ) as HTMLElement[];
    expect(afterGotoFrames[0]).toBe(initialFrames[0]);
    expect(afterGotoFrames[1]).toBe(initialFrames[1]);
    expect(afterGotoFrames[0]?.dataset.active).toBe('false');
    expect(afterGotoFrames[1]?.dataset.active).toBe('true');
    dom.window.close();
  });

  it('renders current-thread draft markers only after a validated session command', () => {
    const dom = createRuntime();
    const { window } = dom;
    window.dispatchEvent(new window.MessageEvent('message', { source: window as unknown as Window, data: { type: 'updateDeck', slides: [{ id: 's1', html: '<h1>one</h1>', frame: frame('s1', 1) }], index: 0 } }));
    window.dispatchEvent(new window.MessageEvent('message', { source: window as unknown as Window, data: { type: 'setSelectionMode', session_id: 'session-one', slide_id: 's1', mode: 'element', html_revision: 1, html_hash: 'sha256:a' } }));
    window.dispatchEvent(new window.MessageEvent('message', { source: window as unknown as Window, data: { type: 'renderDraftSelections', session_id: 'session-one', slide_id: 's1', selections: [{ selection_id: 'sel_one', marker_no: 3, status: 'active', rect: { x: 10, y: 20, width: 100, height: 40 } }] } }));
    expect(window.document.querySelector('.selection-box span')?.textContent).toBe('3');
    dom.window.close();
  });

  it('appends prefetched slides without rebuilding existing frames', () => {
    const dom = createRuntime();
    const { window } = dom;

    window.dispatchEvent(new window.MessageEvent('message', {
      source: window as unknown as Window,
      data: {
        type: 'updateDeck',
        slides: [{ id: 's1', html: '<!doctype html><title>one</title>', frame: frame('s1', 1) }],
        index: 0,
      },
    }));
    const firstFrame = window.document.querySelector('[data-slide-frame]') as HTMLElement;

    window.dispatchEvent(new window.MessageEvent('message', {
      source: window as unknown as Window,
      data: {
        type: 'updateDeck',
        slides: [
          { id: 's1', html: '<!doctype html><title>one</title>', frame: frame('s1', 1) },
          { id: 's2', html: '<!doctype html><title>two</title>', frame: frame('s2', 2) },
        ],
        index: 0,
      },
    }));

    const frames = Array.from(
      window.document.querySelectorAll('[data-slide-frame]'),
    ) as HTMLElement[];
    expect(frames).toHaveLength(2);
    expect(frames[0]).toBe(firstFrame);
    expect(frames[0]?.dataset.active).toBe('true');
    expect(frames[1]?.dataset.active).toBe('false');
    dom.window.close();
  });

  it('ignores undeclared payloads and messages not sent by the parent', () => {
    const dom = createRuntime();
    const { window } = dom;
    const validDeck = {
      type: 'updateDeck',
      slides: [{ id: 's1', html: '<h1>safe</h1>', frame: frame('s1', 1) }],
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
