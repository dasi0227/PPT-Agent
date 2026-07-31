import { describe, expect, it } from 'vitest';
import type { Slide } from '../../api/types';
import { createTargetedRun } from './modeMapping';

const slide = { id: 'stable-slide', project_id: 'p1', idx: 0, layout: 'content', title: 'S', html_path: '', json_path: '', current_version: 0, order: 0, outline_dirty: false } satisfies Slide;

describe('createTargetedRun', () => {
  it('resolves the current page to a stable slide id', () => {
    expect(createTargetedRun({
      artifact: 'presentation', level: 'slide', intent: 'apply', instruction: 'revise',
      slides: [slide], currentPage: 0,
    })).toEqual({
      target: { artifact: 'presentation', level: 'slide', slide_id: 'stable-slide' },
      interaction: { intent: 'apply', clarification: 'when_blocked' },
      instruction: 'revise',
    });
  });

  it('falls back to deck when no page exists', () => {
    expect(createTargetedRun({
      artifact: 'blueprint', level: 'slide', intent: 'consult', instruction: 'advise',
      slides: [], currentPage: 0,
    }).target).toEqual({ artifact: 'blueprint', level: 'deck' });
  });

  it.each([
    ['blueprint', 'deck'], ['blueprint', 'slide'], ['presentation', 'deck'], ['presentation', 'slide'],
  ] as const)('supports %s/%s', (artifact, level) => {
    expect(createTargetedRun({ artifact, level, intent: 'apply', instruction: 'go', slides: [slide], currentPage: 0 }).target.artifact).toBe(artifact);
  });
});
