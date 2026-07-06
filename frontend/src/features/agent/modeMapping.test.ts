import { describe, expect, it } from 'vitest';
import { mapModeToPayload, smartDefault, MapInput } from './modeMapping';

function base(overrides: Partial<MapInput>): MapInput {
  return {
    interactionMode: 'outline',
    subMode: 'normal',
    hasOutline: false,
    currentPageHasHtml: false,
    currentPage: 0,
    targetPageIndex: null,
    instruction: 'do it',
    ...overrides,
  };
}

describe('smartDefault', () => {
  it('empty project defaults to outline', () => {
    expect(smartDefault(false)).toBe('outline');
  });
  it('project with outline defaults to page', () => {
    expect(smartDefault(true)).toBe('page');
  });
});

describe('mapModeToPayload — matrix rows', () => {
  it('Outline / normal / no outline → kind=outline scope=current', () => {
    const p = mapModeToPayload(base({
      interactionMode: 'outline', subMode: 'normal', hasOutline: false,
      outlineOpts: { brief: 'b', slide_count: 8, language: 'zh' },
    }));
    expect(p).toMatchObject({ kind: 'outline', scope: 'current', mode: 'normal', brief: 'b', slide_count: 8, language: 'zh' });
    expect(p.instruction).toBe('do it');
  });

  it('Outline / normal / has outline → kind=edit scope=overview (方案A)', () => {
    const p = mapModeToPayload(base({ interactionMode: 'outline', subMode: 'normal', hasOutline: true }));
    expect(p).toMatchObject({ kind: 'edit', scope: 'overview', mode: 'normal' });
  });

  it('Outline / talk → kind=command command=talk mode=talk scope=current', () => {
    const p = mapModeToPayload(base({ interactionMode: 'outline', subMode: 'talk', hasOutline: false }));
    expect(p).toMatchObject({ kind: 'command', command: 'talk', mode: 'talk', scope: 'current' });
  });

  it('Outline / ask / no outline → kind=outline mode=ask', () => {
    const p = mapModeToPayload(base({ interactionMode: 'outline', subMode: 'ask', hasOutline: false }));
    expect(p).toMatchObject({ kind: 'outline', scope: 'current', mode: 'ask' });
  });

  it('Page / normal / current page no html → kind=generate scope=current page_index=current', () => {
    const p = mapModeToPayload(base({
      interactionMode: 'page', subMode: 'normal', hasOutline: true,
      currentPageHasHtml: false, currentPage: 2, targetPageIndex: null,
    }));
    expect(p).toMatchObject({ kind: 'generate', scope: 'current', mode: 'normal', page_index: 2 });
  });

  it('Page / normal / current page has html → kind=edit scope=current', () => {
    const p = mapModeToPayload(base({
      interactionMode: 'page', subMode: 'normal', hasOutline: true,
      currentPageHasHtml: true, currentPage: 1, targetPageIndex: null,
    }));
    expect(p).toMatchObject({ kind: 'edit', scope: 'current', mode: 'normal', page_index: 1 });
  });

  it('Page（指定页）/ normal → kind=edit scope=page page_index=指定', () => {
    const p = mapModeToPayload(base({
      interactionMode: 'page', subMode: 'normal', hasOutline: true,
      currentPage: 0, targetPageIndex: 4,
    }));
    expect(p).toMatchObject({ kind: 'edit', scope: 'page', mode: 'normal', page_index: 4 });
  });

  it('Page / ask → kind=command command=ask scope=current/page', () => {
    const p = mapModeToPayload(base({
      interactionMode: 'page', subMode: 'ask', hasOutline: true, currentPage: 3, targetPageIndex: null,
    }));
    expect(p).toMatchObject({ kind: 'command', command: 'ask', mode: 'ask', scope: 'current', page_index: 3 });
  });

  it('Page / talk → kind=command command=talk scope=current', () => {
    const p = mapModeToPayload(base({ interactionMode: 'page', subMode: 'talk', hasOutline: true }));
    expect(p).toMatchObject({ kind: 'command', command: 'talk', mode: 'talk', scope: 'current' });
  });

  it('Overview / normal → kind=edit scope=overview', () => {
    const p = mapModeToPayload(base({ interactionMode: 'overview', subMode: 'normal', hasOutline: true }));
    expect(p).toMatchObject({ kind: 'edit', scope: 'overview', mode: 'normal' });
  });

  it('Overview / talk → kind=command command=talk scope=current', () => {
    const p = mapModeToPayload(base({ interactionMode: 'overview', subMode: 'talk', hasOutline: true }));
    expect(p).toMatchObject({ kind: 'command', command: 'talk', mode: 'talk', scope: 'current' });
  });

  it('Repo / normal → kind=edit scope=repo', () => {
    const p = mapModeToPayload(base({ interactionMode: 'repo', subMode: 'normal' }));
    expect(p).toMatchObject({ kind: 'edit', scope: 'repo', mode: 'normal' });
  });

  it('Repo / talk → kind=command command=talk scope=current', () => {
    const p = mapModeToPayload(base({ interactionMode: 'repo', subMode: 'talk' }));
    expect(p).toMatchObject({ kind: 'command', command: 'talk', mode: 'talk', scope: 'current' });
  });
});
