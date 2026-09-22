import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { JSDOM } from 'jsdom';
import { describe, expect, it } from 'vitest';

const asset = (path: string) => readFileSync(resolve(process.cwd(), '../backend/internal', path), 'utf8');
const sharedChrome = asset('runtimeassets/chrome.js');
const player = asset('export/player/player.js');
const css = asset('export/player/player.css');
const context = {
  ordinal: 1, total: 2, numbering: { visible: false }, section: { title: '开场' }, deck_title: 'Deck',
  chrome: [
    { type: 'section_marker', placement: 'top-left', style: 'compact label' },
    { type: 'deck_title', placement: 'bottom-left', style: 'tiny mono' },
    { type: 'page_number', placement: 'bottom-right', style: 'tiny mono' },
  ],
};

function createPlayer() {
  const slides = [
    { src: 'slides/001.html', frame: context },
    { src: 'slides/002.html', frame: { ...context, ordinal: 2, numbering: { visible: true } } },
  ];
  const html = asset('export/player/index.html').replace('%s', 'Deck').replace('%s', () => JSON.stringify(slides));
  const dom = new JSDOM(html, { runScripts: 'outside-only', url: 'file:///deck/index.html' });
  const style = dom.window.document.createElement('style');
  style.textContent = css;
  dom.window.document.head.appendChild(style);
  dom.window.eval(`window.__PPT_SLIDES__=${JSON.stringify(slides)}`);
  dom.window.eval(sharedChrome);
  return dom;
}

describe('standalone HTML player', () => {
  it('renders the preview chrome outside the slide without adding any player controls', () => {
    const dom = createPlayer();
    try {
      dom.window.eval(player);
      const { document } = dom.window;
      const marker = document.querySelector('[data-runtime-chrome="section_marker"]')!;
      const title = document.querySelector('[data-runtime-chrome="deck_title"]')!;
      expect(marker.textContent).toBe('开场');
      expect(marker.parentElement?.id).toBe('canvas');
      expect(dom.window.getComputedStyle(marker).position).toBe('absolute');
      expect(dom.window.getComputedStyle(marker).fontSize).toBe('16px');
      expect(dom.window.getComputedStyle(title).bottom).toBe('3.2%');
      expect(document.querySelector('[data-runtime-page-number]')).toBeNull();
      expect(document.querySelector('iframe')?.getAttribute('sandbox')).toBe('allow-scripts');
      expect(document.querySelector('nav,button,#controls,#counter')).toBeNull();
    } finally { dom.window.close(); }
  });

  it('accepts input only from the active page, preserves boundary execution, and renders new page numbers', () => {
    const dom = createPlayer();
    try {
      const { window } = dom;
      window.eval(player);
      const first = window.document.querySelector('iframe')!;
      const oldSource = first.contentWindow;
      const key = (value: string) => window.dispatchEvent(new window.KeyboardEvent('keydown', { key: value }));
      key('ArrowLeft');
      expect(window.document.querySelector('iframe')).toBe(first);
      window.dispatchEvent(new window.MessageEvent('message', {
        source: oldSource, data: { type: 'ppt-player-input', key: 'ArrowRight' },
      }));
      expect(window.document.querySelector('iframe')?.getAttribute('src')).toBe('slides/002.html');
      expect(window.document.querySelector('[data-runtime-page-number]')?.textContent).toBe('2');
      window.dispatchEvent(new window.MessageEvent('message', {
        source: oldSource, data: { type: 'ppt-player-input', key: 'Home' },
      }));
      expect(window.document.querySelector('iframe')?.getAttribute('src')).toBe('slides/002.html');
      key('Home');
      expect(window.document.querySelector('[data-runtime-page-number]')).toBeNull();
    } finally { dom.window.close(); }
  });
});
