import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { JSDOM } from 'jsdom';
import { describe, expect, it } from 'vitest';

const asset = (path: string) => readFileSync(resolve(process.cwd(), '../backend/internal', path), 'utf8');
const sharedDecorations = asset('runtimeassets/decorations.js');
const player = asset('export/player/player.js');
const css = asset('export/player/player.css');
const context = {
  appearance: { hash: "snapshot-appearance", decoration_tokens: { "--color-caption": "#78695b", "--color-fg": "#302820", "--font-sans": "Noto Sans SC", "--font-mono": "JetBrains Mono" } },
  ordinal: 1, total: 2, section: { title: '开场' }, deck_title: 'Deck',
  decorations: { slide_pagination: 'bottom-right', deck_title: 'bottom-left', section_title: 'top-left', logo: 'none' },
};

function createPlayer(decorations = context.decorations, logo: { src: string; hash: string } | null = null) {
  const slides = [
    { src: 'slides/001.html', frame: { ...context, logo, decorations } },
    { src: 'slides/002.html', frame: { ...context, logo, decorations, ordinal: 2 } },
  ];
  const html = asset('export/player/index.html').replace('%s', 'Deck').replace('%s', () => JSON.stringify(slides));
  const dom = new JSDOM(html, { runScripts: 'outside-only', url: 'file:///deck/index.html' });
  const style = dom.window.document.createElement('style');
  style.textContent = css;
  dom.window.document.head.appendChild(style);
  dom.window.eval(`window.__PPT_SLIDES__=${JSON.stringify(slides)}`);
  dom.window.eval(sharedDecorations);
  return dom;
}

describe('standalone HTML player', () => {
  it('renders the preview decorations outside the slide without adding any player controls', () => {
    const dom = createPlayer();
    try {
      dom.window.eval(player);
      const { document } = dom.window;
      const marker = document.querySelector('[data-runtime-decoration="section_title"]')!;
      const title = document.querySelector('[data-runtime-decoration="deck_title"]')!;
      expect(marker.textContent).toBe('开场');
      expect(marker.parentElement?.id).toBe('canvas');
      expect(dom.window.getComputedStyle(marker).position).toBe('absolute');
      expect(dom.window.getComputedStyle(marker).fontSize).toBe('16px');
      expect(dom.window.getComputedStyle(title).bottom).toBe('3.2%');
      expect(document.querySelector('[data-runtime-slide-pagination]')?.textContent).toBe('1');
      expect(document.querySelectorAll('[data-runtime-slide-pagination]')).toHaveLength(1);
      expect(document.querySelector('iframe')?.getAttribute('sandbox')).toBe('allow-scripts');
      expect(document.querySelector('nav,button,#controls,#counter')).toBeNull();
    } finally { dom.window.close(); }
  });

  it('shows only the initialized page number when optional decorations are disabled', () => {
    const dom = createPlayer({ slide_pagination: 'bottom-right', deck_title: 'none', section_title: 'none', logo: 'none' });
    try {
      dom.window.eval(player);
      const number = dom.window.document.querySelector('[data-runtime-slide-pagination]');
      expect(number?.textContent).toBe('1');
      expect(dom.window.document.querySelectorAll('[data-runtime-decoration]')).toHaveLength(1);
      expect(number?.getAttribute('data-placement')).toBe('bottom-right');
      dom.window.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'ArrowRight' }));
      expect(dom.window.document.querySelector('[data-runtime-slide-pagination]')?.textContent).toBe('2');
      expect(dom.window.document.querySelectorAll('[data-runtime-slide-pagination]')).toHaveLength(1);
    } finally { dom.window.close(); }
  });


  it('renders the shared logo at its fixed size only when configured and uploaded', () => {
    const decorations = { ...context.decorations, logo: 'bottom-center' };
    const logo = { src: 'branding/logo.png', hash: 'logo-image' };
    const dom = createPlayer(decorations, logo);
    try {
      dom.window.eval(player);
      const marker = dom.window.document.querySelector<HTMLElement>('[data-runtime-decoration="logo"]')!;
      expect(marker.dataset.placement).toBe('bottom-center');
      expect(dom.window.getComputedStyle(marker).width).toBe('128px');
      expect(dom.window.getComputedStyle(marker).height).toBe('64px');
      expect(marker.querySelector('img')?.getAttribute('src')).toBe(logo.src);
      expect(dom.window.getComputedStyle(marker.querySelector('img')!).objectFit).toBe('contain');
      const canvas = dom.window.document.querySelector('#canvas');
      dom.window.PPTDecorations.render(canvas, { ...context, decorations, logo: null });
      expect(canvas?.querySelector('[data-runtime-decoration="logo"]')).toBeNull();
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
      expect(window.document.querySelector('[data-runtime-slide-pagination]')?.textContent).toBe('2');
      window.dispatchEvent(new window.MessageEvent('message', {
        source: oldSource, data: { type: 'ppt-player-input', key: 'Home' },
      }));
      expect(window.document.querySelector('iframe')?.getAttribute('src')).toBe('slides/002.html');
      key('Home');
      expect(window.document.querySelector('[data-runtime-slide-pagination]')?.textContent).toBe('1');
    } finally { dom.window.close(); }
  });
});
