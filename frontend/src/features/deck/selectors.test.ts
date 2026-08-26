import { describe, expect, it } from 'vitest';
import type { Outline, ProjectContentSnapshot } from '../../api/types';
import { adjacentSlideIds, flattenOutline, ordinalBySlideId, orderedSlides, selectedSlide } from './selectors';

const outline: Outline = {
  version: '4.0', revision: 3, project_id: 'pro_1', created_at: 1, updated_at: 2,
  sections: [
    { id: 'sec_a', title: '开场', purpose: '建立主题', slides: [
      { slide_id: 'sli_1', label: '封面', role: 'cover' },
      { slide_id: 'sli_2', label: '议程', role: 'agenda' },
    ], subsections: [] },
    { id: 'sec_b', title: '主体', purpose: '展开论证', slides: [], subsections: [
      { id: 'sub_b1', title: '原则', slides: [{ slide_id: 'sli_3', label: '原则一', role: 'content' }] },
      { id: 'sub_b2', title: '案例', slides: [{ slide_id: 'sli_4', label: '案例', role: 'evidence' }] },
    ] },
  ],
};

const snapshot: ProjectContentSnapshot = {
  deck: { version: '4.0', revision: 1, project_id: 'pro_1', title: 'Deck', goal: '', audience: '', language: 'zh-CN', requirements: [], prohibitions: [], canvas: { aspect_ratio: '16:9' }, numbering: { enabled: true, hidden_roles: ['cover'], format: 'number' }, created_at: 1, updated_at: 1 },
  outline,
  design: { version: '4.0', revision: 1, project_id: 'pro_1', theme: 'default', direction: '', density: 'medium', chrome: [], created_at: 1, updated_at: 1 },
  slides_by_id: {
    sli_1: { spec_state: 'pending', spec: null, html_state: 'pending', html_revision: 0, materialization: null },
    sli_2: { spec_state: 'pending', spec: null, html_state: 'not_materialized', html_revision: 0, materialization: null },
    sli_3: { spec_state: 'pending', spec: null, html_state: 'not_materialized', html_revision: 0, materialization: null },
    sli_4: { spec_state: 'pending', spec: null, html_state: 'not_materialized', html_revision: 0, materialization: null },
  },
};

describe('canonical outline selectors', () => {
  it('flattens direct and grouped sections as the only page order', () => {
    expect(flattenOutline(outline).map((item) => item.node.slide_id)).toEqual(['sli_1', 'sli_2', 'sli_3', 'sli_4']);
    expect(ordinalBySlideId(outline)).toEqual({ sli_1: 1, sli_2: 2, sli_3: 3, sli_4: 4 });
  });

  it('moves an entire section subtree without changing stable selection', () => {
    const moved = { ...outline, sections: [outline.sections[1], outline.sections[0]] };
    expect(flattenOutline(moved).map((item) => item.node.slide_id)).toEqual(['sli_3', 'sli_4', 'sli_1', 'sli_2']);
    expect(selectedSlide({ ...snapshot, outline: moved }, 'sli_2')?.id).toBe('sli_2');
  });

  it('derives navigation and pending slides without a second ordered array', () => {
    expect(adjacentSlideIds(outline, 'sli_3')).toEqual({ previous: 'sli_2', next: 'sli_4' });
    expect(orderedSlides(snapshot).map((slide) => [slide.id, slide.materialization?.state])).toEqual([
      ['sli_1', 'pending'], ['sli_2', 'not_materialized'], ['sli_3', 'not_materialized'], ['sli_4', 'not_materialized'],
    ]);
  });
});
