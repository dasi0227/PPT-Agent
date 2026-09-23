import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { JSDOM } from 'jsdom';
import { describe, expect, it, vi } from 'vitest';

const source = readFileSync(resolve(process.cwd(), '../backend/internal/runtimeassets/theme-bridge.js'), 'utf8');
const flush = () => new Promise(resolve => setTimeout(resolve, 0));
function fixture() {
  const load = vi.fn().mockResolvedValue([{}]);
  const dom = new JSDOM(`<html><head><link id="base-link" rel="stylesheet" href="/api/v1/runtime/base.css"><link id="theme-link" rel="stylesheet" href="/old.css"><style id="page-style">.custom { margin:20px }</style></head><body><p id="state">kept</p><script>window.executions=(window.executions||0)+1</script><script data-slide-id="s1">${source}</script></body></html>`, {
    url: 'http://localhost/slide', runScripts: 'dangerously',
    beforeParse(window) { window.PPTFonts={prepare:load}; },
  });
  const messages = vi.spyOn(dom.window, 'postMessage');
  const original = dom.window.document.getElementById('theme-link');
  const apply = (request: number) => {
    dom.window.dispatchEvent(new dom.window.MessageEvent('message', { source: dom.window, data: {
      bridge:'ppt-theme-v1',type:'applyTheme',slide_id:'s1',request_id:request,theme_id:`theme-${request}`,
      appearance:{hash:`hash-${request}`,theme_css_url:`/theme-${request}.css`,chrome_tokens:{}},
    } }));
    return dom.window.document.querySelector(`link[href="/theme-${request}.css"]`)!;
  };
  const finish = async (link: Element, type = 'load') => {
    dom.window.document.querySelectorAll('link[href="/api/v1/runtime/base.css"]:not([id])').forEach(base=>base.dispatchEvent(new dom.window.Event('load')));
    link.dispatchEvent(new dom.window.Event(type)); await flush(); };
  return { dom, messages, load, original, apply, finish };
}

describe('resource-only theme application', () => {
  it('commits only the latest request, retaining the old stylesheet until ready and preserving page state', async () => {
    const f=fixture();
    try {
      const first=f.apply(1), latest=f.apply(2);
      await f.finish(first);
      expect(f.dom.window.document.getElementById('theme-link')).toBe(f.original);
      await f.finish(latest);
      expect(f.dom.window.document.getElementById('theme-link')).toBe(latest);
      expect(latest.nextElementSibling?.id).toBe('page-style');
      expect(f.dom.window.document.getElementById('state')?.textContent).toBe('kept');
      expect(f.dom.window.executions).toBe(1);
      const applied=f.messages.mock.calls.map(([message])=>message).filter(message=>message.type==='themeApplied');
      expect(applied.map(message=>message.appearance_hash)).toEqual(['hash-2']);
    } finally { f.dom.window.close(); }
  });

  it.each(['css','font'])('preserves the current theme on %s failure and retries without replacing the document', async failure => {
    const f=fixture();
    try {
      if(failure==='font')f.load.mockRejectedValue(new Error('font unavailable'));
      const link=f.apply(1);
      await f.finish(link,failure==='css'?'error':'load');
      expect(f.dom.window.document.getElementById('theme-link')).toBe(f.original);
      expect(f.messages.mock.calls.some(([message])=>message.type==='themeApplyFailed')).toBe(true);
      f.load.mockResolvedValue([{}]);
      const retried=f.apply(2);await f.finish(retried);
      expect(f.dom.window.document.getElementById('theme-link')).toBe(retried);
      expect(f.dom.window.executions).toBe(1);
    } finally { f.dom.window.close(); }
  });
});
