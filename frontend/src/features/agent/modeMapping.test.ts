import { describe, expect, it } from 'vitest';
import type { Slide } from '../../api/types';
import { createTargetedRun } from './modeMapping';

const slide = { id: 'stable-slide', project_id: 'p1', layout: 'content', title: 'S', html_path: '', spec_path: '', html_hash: '' } satisfies Slide;

describe('createTargetedRun', () => {
  it('resolves the current page to a stable slide id', () => {
    expect(createTargetedRun({
      selection: 'current_page', mode: 'execute', instruction: 'revise',
      slides: [slide], currentSlideId: 'stable-slide',
    })).toEqual({
      scope: { selection: { kind: 'current_page', current_slide_id: 'stable-slide' } },
      mode: 'execute',
      instruction: 'revise',
    });
  });

  it('falls back to deck when no page exists', () => {
    expect(createTargetedRun({
      selection: 'current_page', mode: 'chat', instruction: 'advise',
      slides: [], currentSlideId: null,
    }).scope).toEqual({ selection: { kind: 'all_pages' } });
  });

});
