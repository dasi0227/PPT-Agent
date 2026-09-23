import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it, vi } from 'vitest';
import { JSDOM } from 'jsdom';

function createRuntime() {
  const html = readFileSync(
    resolve(process.cwd(), 'public/slide-runtime/index.html'),
    'utf8',
  );
  const chrome = readFileSync(resolve(process.cwd(), '../backend/internal/runtimeassets/chrome.js'), 'utf8');
  return new JSDOM(html.replace('<script src="/api/v1/runtime/chrome.js"></script>', () => `<script>${chrome}</script>`), {
    runScripts: 'dangerously',
    url: 'http://localhost/slide-runtime/index.html',
  });
}

async function applyCurrentTheme(window: ReturnType<typeof createRuntime>['window']) {
  window.PPTFonts={prepare:()=>Promise.resolve()};
  const iframe=window.document.querySelector('iframe')!;
  const spy=vi.spyOn(iframe.contentWindow!,'postMessage');
  const slideID=window.document.querySelector('[data-slide-frame]')!.getAttribute('data-slide-id');
  window.dispatchEvent(new window.MessageEvent('message',{source:iframe.contentWindow,data:{bridge:'ppt-theme-v1',type:'themeBridgeReady',slide_id:slideID}}));
  await new Promise(resolve=>setTimeout(resolve,0));
  const request=spy.mock.calls.map(([message])=>message).reverse().find(message=>message.type==='applyTheme');
  window.dispatchEvent(new window.MessageEvent('message',{source:iframe.contentWindow,data:{bridge:'ppt-theme-v1',type:'themeApplied',slide_id:slideID,request_id:request.request_id,appearance_hash:request.appearance.hash}}));
  spy.mockRestore();
}

describe('slide runtime', () => {
  const frame = (id: string, ordinal: number, visible = true) => ({
    slide_id: id, ordinal, total: 2, role: ordinal === 1 ? 'cover' : 'content',
    canvas: { width: 1920, height: 1080, aspect_ratio: '16:9' },
    theme_id: 'editorial-serif', appearance: {hash:'appearance-1',theme_css_url:'/api/v1/themes/editorial-serif/css',chrome_tokens:{}},
    section: { id: 'sec_1', title: '正文', index: 1 }, numbering: { visible, format: 'number' },
    deck_title: 'Deck',
    chrome: [
      { type: 'page_number', placement: 'bottom-right', style: 'tiny muted mono counter' },
      { type: 'section_marker', placement: 'top-left', style: 'muted label' },
      { type: 'deck_title', placement: 'top-right', style: 'muted label' },
    ],
  });

  const send = (
    runtimeWindow: ReturnType<typeof createRuntime>['window'],
    data: unknown,
    source: MessageEventSource = runtimeWindow as unknown as Window,
  ) => {
    runtimeWindow.dispatchEvent(new runtimeWindow.MessageEvent('message', { source, data }));
  };

  it('executes only the current slide and creates fresh frames when navigating away and back', async () => {
    const dom = createRuntime();
    const { window } = dom;
    const slides = [
      { id: 's1', html: '<!doctype html><title>one</title>', frame: frame('s1', 1, false) },
      { id: 's2', html: '<!doctype html><title>two</title>', frame: frame('s2', 2) },
    ];

    send(window, { type: 'updateDeck', slides, index: 0 });

    const initialFrames = Array.from(
      window.document.querySelectorAll('[data-slide-frame]'),
    ) as HTMLElement[];

    expect(initialFrames).toHaveLength(1);
    const initialFrame = initialFrames[0];
    const srcdoc = initialFrame.querySelector('iframe')?.getAttribute('srcdoc') ?? '';
    expect(srcdoc).toContain('id="base-link"');
    expect(srcdoc).toContain('/api/v1/themes/editorial-serif/css');
    expect(srcdoc).toContain('/slide-runtime/selection-bridge.js');
    expect(srcdoc).not.toContain('<title>two</title>');
    expect(initialFrames[0]?.querySelector('.runtime-canvas')).not.toBeNull();
    expect(initialFrames.every((container) => container.querySelector('iframe')?.getAttribute('sandbox') === 'allow-scripts')).toBe(true);
    expect(initialFrames[0]?.querySelector('[data-runtime-page-number]')).toBeNull();
    expect(initialFrames[0]?.dataset.active).toBe('true');

    send(window, { type: 'gotoSlide', index: 1 });

    await applyCurrentTheme(window);
    const afterGotoFrames = Array.from(
      window.document.querySelectorAll('[data-slide-frame]'),
    ) as HTMLElement[];
    expect(afterGotoFrames).toHaveLength(1);
    expect(afterGotoFrames[0]).not.toBe(initialFrame);
    expect(afterGotoFrames[0]?.dataset.slideId).toBe('s2');
    expect(afterGotoFrames[0]?.querySelector('[data-runtime-page-number]')?.textContent).toBe('2');
    expect(afterGotoFrames[0]?.querySelector('[data-runtime-chrome="section_marker"]')?.textContent).toBe('正文');
    expect(afterGotoFrames[0]?.querySelector('[data-runtime-chrome="deck_title"]')?.textContent).toBe('Deck');

    send(window, { type: 'gotoSlide', index: 0 });
    const returnedFrame = window.document.querySelector('[data-slide-frame]') as HTMLElement;
    expect(returnedFrame.dataset.slideId).toBe('s1');
    expect(returnedFrame).not.toBe(initialFrame);
    dom.window.close();
  });

  it('renders current-thread draft markers only after a validated session command', () => {
    const dom = createRuntime();
    const { window } = dom;
    send(window, { type: 'updateDeck', slides: [{ id: 's1', html: '<h1>one</h1>', frame: frame('s1', 1) }], index: 0 });
    send(window, { type: 'setSelectionMode', session_id: 'session-one', slide_id: 's1', mode: 'element', html_hash: 'sha256:a' });
    send(window, { type: 'renderDraftSelections', session_id: 'session-one', slide_id: 's1', selections: [{ selection_id: 'sel_one', marker_no: 3, status: 'active', rect: { x: 10, y: 20, width: 100, height: 40 } }] });
    expect(window.document.querySelector('.selection-box span')?.textContent).toBe('3');

    send(window, { type: 'updateDeck', slides: [{ id: 's1', html: '<h1>updated</h1>', frame: frame('s1', 1) }], index: 0 });
    expect(window.document.querySelector('.selection-box span')?.textContent).toBe('3');
    dom.window.close();
  });

  it('keeps the current execution for prefetch, repeated navigation, and frame metadata updates', async () => {
    const dom = createRuntime();
    const { window } = dom;

    send(window, {
      type: 'updateDeck',
      slides: [{ id: 's1', html: '<!doctype html><title>one</title>', frame: frame('s1', 1) }],
      index: 0,
    });
    await applyCurrentTheme(window);
    const firstFrame = window.document.querySelector('[data-slide-frame]') as HTMLElement;
    send(window, { type: 'setSelectionMode', session_id: 'session-one', slide_id: 's1', mode: 'element', html_hash: 'sha256:a' });

    const updatedFrame = {
      ...frame('s1', 1),
      ordinal: 2,
      total: 3,
      deck_title: 'Renamed Deck',
    };
    send(window, {
      type: 'updateDeck',
      slides: [
        { id: 's1', html: '<!doctype html><title>one</title>', frame: updatedFrame },
        { id: 's2', html: '<!doctype html><title>prefetched</title>', frame: frame('s2', 2) },
      ],
      index: 0,
    });
    send(window, { type: 'gotoSlide', index: 0 });

    const frames = Array.from(
      window.document.querySelectorAll('[data-slide-frame]'),
    ) as HTMLElement[];
    expect(frames).toHaveLength(1);
    expect(frames[0]).toBe(firstFrame);
    expect(frames[0]?.dataset.active).toBe('true');
    expect(frames[0]?.querySelector('[data-runtime-page-number]')?.textContent).toBe('2');
    expect(frames[0]?.querySelector('[data-runtime-chrome="deck_title"]')?.textContent).toBe('Renamed Deck');
    expect((frames[0]?.querySelector('[data-runtime-page-number]') as HTMLElement)?.style.pointerEvents).toBe('auto');
    expect(frames[0]?.querySelector('iframe')?.getAttribute('srcdoc')).not.toContain('prefetched');
    dom.window.close();
  });

  it('keeps the iframe for theme changes and rebuilds only for HTML or matching replay', () => {
    const dom = createRuntime();
    const { window } = dom;
    const html = '<!doctype html><title>one</title>';
    const slide = { id: 's1', html, frame: frame('s1', 1) };

    send(window, { type: 'updateDeck', slides: [slide], index: 0 });
    const initial = window.document.querySelector('[data-slide-frame]');

    send(window, { type: 'updateDeck', slides: [{ ...slide, html: '<!doctype html><title>changed</title>' }], index: 0 });
    const htmlChanged = window.document.querySelector('[data-slide-frame]');
    expect(htmlChanged).not.toBe(initial);

    const themed = { ...slide, html: '<!doctype html><title>changed</title>', frame: { ...slide.frame, theme_id: 'blueprint',appearance:{hash:'appearance-2',theme_css_url:'/api/v1/themes/blueprint/css',chrome_tokens:{}} } };
    send(window, { type: 'updateDeck', slides: [themed], index: 0 });
    const themeChanged = window.document.querySelector('[data-slide-frame]');
    expect(themeChanged).toBe(htmlChanged);

    send(window, { type: 'replayCurrentSlide', slide_id: 'other' });
    expect(window.document.querySelector('[data-slide-frame]')).toBe(themeChanged);
    send(window, { type: 'replayCurrentSlide', slide_id: 's1', callback: 'x' });
    expect(window.document.querySelector('[data-slide-frame]')).toBe(themeChanged);
    send(window, { type: 'replayCurrentSlide', slide_id: 's1' });
    expect(window.document.querySelector('[data-slide-frame]')).not.toBe(themeChanged);
    dom.window.close();
  });

  it('ignores obsolete completion and retains the iframe on failure and retry', async () => {
    const dom=createRuntime(),window=dom.window;
    const slide={id:'s1',html:'<p>persistent</p>',frame:frame('s1',1)};
    send(window,{type:'updateDeck',slides:[slide],index:0});
    await applyCurrentTheme(window);
    const iframe=window.document.querySelector('iframe')!;
    const requests=vi.spyOn(iframe.contentWindow!,'postMessage');
    const events=vi.spyOn(window,'postMessage');
    const changed={...slide,frame:{...slide.frame,appearance:{...slide.frame.appearance,hash:'next',theme_css_url:'/next.css'}}};
    send(window,{type:'updateDeck',slides:[changed],index:0});
    await new Promise(resolve=>setTimeout(resolve,0));
    const request=requests.mock.calls.map(([message])=>message).reverse().find(message=>message.type==='applyTheme');
    const reply=(type:string,requestID:number,hash='next')=>send(window,{bridge:'ppt-theme-v1',type,request_id:requestID,appearance_hash:hash,slide_id:'s1',message:'load failed'},iframe.contentWindow! as unknown as Window);
    reply('themeApplied',request.request_id-1,'old');
    expect(events.mock.calls.some(([message])=>message.type==='themeApplied'&&message.appearance_hash==='old')).toBe(false);
    reply('themeApplyFailed',request.request_id);
    expect(window.document.querySelector('iframe')).toBe(iframe);
    expect(events.mock.calls.some(([message])=>message.type==='themeApplyFailed')).toBe(true);
    send(window,{type:'retryTheme',slide_id:'s1'});
    await new Promise(resolve=>setTimeout(resolve,0));
    const retry=requests.mock.calls.map(([message])=>message).reverse().find(message=>message.type==='applyTheme');
    expect(retry.request_id).toBeGreaterThan(request.request_id);
    reply('themeApplied',retry.request_id);
    expect(window.document.querySelector('iframe')).toBe(iframe);
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

    send(window, validDeck, {} as Window);
    send(window, { type: 'executeScript', callback: 'alert(1)' });

    expect(window.document.querySelectorAll('[data-slide-frame]')).toHaveLength(0);
    dom.window.close();
  });
});
