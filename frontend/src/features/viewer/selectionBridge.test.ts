import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import { JSDOM } from 'jsdom';

function createBridge(body: string) {
  const bridge = readFileSync(resolve(process.cwd(), 'public/slide-runtime/selection-bridge.js'), 'utf8');
  const messages: Array<Record<string, unknown>> = [];
  const dom = new JSDOM(`<!doctype html><body>${body}<script data-slide-id="sli_one">${bridge}</script></body>`, {
    runScripts: 'dangerously',
    beforeParse(window) {
      window.postMessage = ((message: Record<string, unknown>) => { messages.push(message); }) as typeof window.postMessage;
      const escape = (value: string) => value.replace(/[^a-zA-Z0-9_-]/g, '\\$&');
      if (window.CSS) Object.defineProperty(window.CSS, 'escape', { configurable: true, value: escape });
      else Object.defineProperty(window, 'CSS', { configurable: true, value: { escape } });
    },
  });
  return { dom, messages };
}

function enterMode(window: JSDOM['window'], mode: 'element' | 'region') {
  const MessageEventConstructor = window.eval('MessageEvent') as typeof MessageEvent;
  window.dispatchEvent(new MessageEventConstructor('message', {
    source: window as unknown as Window,
    data: {
      bridge: 'ppt-dom-selection-v1', type: 'innerSetSelectionMode', session_id: 'session-one',
      slide_id: 'sli_one', mode, html_revision: 2, html_hash: 'sha256:one',
    },
  }));
}

function setRect(element: Element, left: number, top: number, width: number, height: number) {
  Object.defineProperty(element, 'getBoundingClientRect', {
    configurable: true,
    value: () => ({ left, top, right: left + width, bottom: top + height, width, height, x: left, y: top, toJSON: () => ({}) }),
  });
}

describe('inner DOM selection bridge', () => {
  it('captures a nested iframe as one sanitized DOM target', () => {
    const { dom, messages } = createBridge('<iframe id="nested" srcdoc="<script>secret()</script>" onload="steal()"></iframe>');
    const { window } = dom;
    const target = window.document.querySelector('#nested')!;
    setRect(target, 20, 30, 200, 100);
    Object.defineProperty(window.document, 'elementFromPoint', { configurable: true, value: () => target });
    enterMode(window, 'element');
    window.document.dispatchEvent(new window.MouseEvent('click', { bubbles: true, clientX: 30, clientY: 40 }));

    const created = messages.find((message) => message.type === 'innerSelectionCreated');
    const selection = created?.selection as { dom_targets: Array<{ tag: string; outer_html: string; attributes: Record<string, string> }> };
    expect(selection.dom_targets).toHaveLength(1);
    expect(selection.dom_targets[0]?.tag).toBe('iframe');
    expect(selection.dom_targets[0]?.outer_html).not.toMatch(/srcdoc|onload|secret/);
    expect(selection.dom_targets[0]?.attributes).not.toHaveProperty('srcdoc');
    dom.window.close();
  });

  it('includes only fully contained visible elements in a region', () => {
    const { dom, messages } = createBridge('<div id="inside">inside</div><div id="partial">partial</div>');
    const { window } = dom;
    const inside = window.document.querySelector('#inside')!;
    const partial = window.document.querySelector('#partial')!;
    setRect(inside, 20, 20, 40, 40);
    setRect(partial, 80, 80, 40, 40);
    enterMode(window, 'region');
    window.document.dispatchEvent(new window.MouseEvent('pointerdown', { bubbles: true, button: 0, clientX: 10, clientY: 10 }));
    window.document.dispatchEvent(new window.MouseEvent('pointerup', { bubbles: true, button: 0, clientX: 100, clientY: 100 }));

    const created = messages.find((message) => message.type === 'innerSelectionCreated');
    const selection = created?.selection as { dom_targets: Array<{ attributes: Record<string, string> }> };
    expect(selection.dom_targets.map((target) => target.attributes.id)).toEqual(['inside']);
    dom.window.close();
  });
});
