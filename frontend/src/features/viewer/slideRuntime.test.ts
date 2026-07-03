import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import { JSDOM } from 'jsdom';

describe('slide runtime', () => {
  it('preloads slide frames and switches pages without changing frame src on goto', () => {
    const html = readFileSync(
      resolve(process.cwd(), 'public/slide-runtime/index.html'),
      'utf8'
    );
    const dom = new JSDOM(html, {
      runScripts: 'dangerously',
      url: 'http://localhost/slide-runtime/index.html'
    });

    const { window } = dom;

    window.dispatchEvent(
      new window.MessageEvent('message', {
        origin: 'http://localhost',
        data: {
          type: 'update',
          slides: ['/slides/p1/s1.html', '/slides/p1/s2.html'],
          index: 0
        }
      })
    );

    const initialFrames = Array.from(
      window.document.querySelectorAll('[data-slide-frame]')
    ) as HTMLIFrameElement[];

    expect(initialFrames).toHaveLength(2);
    expect(initialFrames.map((frame) => frame.getAttribute('src'))).toEqual([
      '/slides/p1/s1.html',
      '/slides/p1/s2.html'
    ]);
    expect(initialFrames[0]?.dataset.active).toBe('true');
    expect(initialFrames[1]?.dataset.active).toBe('false');

    window.dispatchEvent(
      new window.MessageEvent('message', {
        origin: 'http://localhost',
        data: { type: 'goto', index: 1 }
      })
    );

    const afterGotoFrames = Array.from(
      window.document.querySelectorAll('[data-slide-frame]')
    ) as HTMLIFrameElement[];

    expect(afterGotoFrames).toHaveLength(2);
    expect(afterGotoFrames.map((frame) => frame.getAttribute('src'))).toEqual([
      '/slides/p1/s1.html',
      '/slides/p1/s2.html'
    ]);
    expect(afterGotoFrames[0]?.dataset.active).toBe('false');
    expect(afterGotoFrames[1]?.dataset.active).toBe('true');

    dom.window.close();
  });
});
