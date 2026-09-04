import { describe, expect, it } from 'vitest';
import type { Slide } from '../../api/types';
import { createTargetedRun } from './modeMapping';

const slide = { id: 'stable-slide', project_id: 'p1', layout: 'content', title: 'S', html_path: '', spec_path: '', current_version: 0 } satisfies Slide;

describe('createTargetedRun', () => {
  it('resolves the current page to a stable slide id', () => {
    expect(createTargetedRun({
      artifact: 'ppt', level: 'slide', mode: 'execute', instruction: 'revise',
      slides: [slide], currentSlideId: 'stable-slide',
    })).toEqual({
      scope: { artifact: 'ppt', level: 'slide', slide_id: 'stable-slide' },
      mode: 'execute',
      instruction: 'revise',
    });
  });

  it('falls back to deck when no page exists', () => {
    expect(createTargetedRun({
      artifact: 'spec', level: 'slide', mode: 'chat', instruction: 'advise',
      slides: [], currentSlideId: null,
    }).scope).toEqual({ artifact: 'spec', level: 'deck' });
  });

  it.each([
    ['spec', 'deck'], ['spec', 'slide'], ['ppt', 'deck'], ['ppt', 'slide'],
  ] as const)('supports %s/%s', (artifact, level) => {
    expect(createTargetedRun({ artifact, level, mode: 'execute', instruction: 'go', slides: [slide], currentSlideId: 'stable-slide' }).scope.artifact).toBe(artifact);
  });
});
