import { describe, expect, it } from 'vitest';
import type { Slide } from '../../api/types';
import { createTargetedRun } from './modeMapping';

const slide = { id: 'stable-slide', project_id: 'p1', position: 0, layout: 'content', title: 'S', html_path: '', spec_path: '', current_version: 0 } satisfies Slide;

describe('createTargetedRun', () => {
  it('resolves the current page to a stable slide id', () => {
    expect(createTargetedRun({
      artifact: 'ppt', level: 'slide', intent: 'execute', instruction: 'revise',
      slides: [slide], currentPage: 0,
    })).toEqual({
      scope: { artifact: 'ppt', level: 'slide', slide_id: 'stable-slide' },
      intent: 'execute',
      instruction: 'revise',
    });
  });

  it('falls back to deck when no page exists', () => {
    expect(createTargetedRun({
      artifact: 'spec', level: 'slide', intent: 'talk', instruction: 'advise',
      slides: [], currentPage: 0,
    }).scope).toEqual({ artifact: 'spec', level: 'deck' });
  });

  it.each([
    ['spec', 'deck'], ['spec', 'slide'], ['ppt', 'deck'], ['ppt', 'slide'],
  ] as const)('supports %s/%s', (artifact, level) => {
    expect(createTargetedRun({ artifact, level, intent: 'execute', instruction: 'go', slides: [slide], currentPage: 0 }).scope.artifact).toBe(artifact);
  });
});
