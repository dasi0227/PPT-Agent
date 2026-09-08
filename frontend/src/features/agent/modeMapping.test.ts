import { describe, expect, it } from 'vitest';
import type { Slide } from '../../api/types';
import { createTargetedRun } from './modeMapping';

const slide = { id: 'stable-slide', project_id: 'p1', layout: 'content', title: 'S', html_path: '', spec_path: '', current_version: 0 } satisfies Slide;

describe('createTargetedRun', () => {
  it('resolves the current page to a stable slide id', () => {
    expect(createTargetedRun({
      object: 'presentation', selection: 'current_page', mode: 'execute', instruction: 'revise',
      slides: [slide], currentSlideId: 'stable-slide',
    })).toEqual({
      scope: { object: 'presentation', selection: { kind: 'current_page', current_slide_id: 'stable-slide' } },
      mode: 'execute',
      instruction: 'revise',
    });
  });

  it('falls back to deck when no page exists', () => {
    expect(createTargetedRun({
      object: 'spec', selection: 'current_page', mode: 'chat', instruction: 'advise',
      slides: [], currentSlideId: null,
    }).scope).toEqual({ object: 'spec', selection: { kind: 'all_pages' } });
  });

  it.each([
    ['spec', 'all_pages'], ['html', 'current_page'], ['presentation', 'all_pages'], ['global', 'all_pages'],
  ] as const)('supports %s/%s', (object, selection) => {
    expect(createTargetedRun({ object, selection, mode: 'execute', instruction: 'go', slides: [slide], currentSlideId: 'stable-slide' }).scope.object).toBe(object);
  });
});
