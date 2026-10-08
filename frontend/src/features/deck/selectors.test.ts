import { describe, expect, it } from 'vitest';
import type { Outline, ProjectContentSnapshot } from '../../api/types';
import { flattenOutline, ordinalBySlideId, orderedSlides } from './selectors';

const outline: Outline = {
  sections: [
    { id: 'sec_a', title: '开场', purpose: '建立主题', slides: [
      { id: 'sli_1', title: '封面' },
      { id: 'sli_2', title: '议程' },
    ], subsections: [] },
    { id: 'sec_b', title: '主体', purpose: '展开论证', slides: [], subsections: [
      { id: 'sub_b1', title: '原则', purpose: '解释原则', slides: [{ id: 'sli_3', title: '原则一' }] },
      { id: 'sub_b2', title: '案例', purpose: '提供论据', slides: [{ id: 'sli_4', title: '案例' }] },
    ] },
  ],
};

const snapshot: ProjectContentSnapshot = {
  project_title: 'Test', project_id: 'pro_1',
  theme: 'default',
  appearance: null,
  hashes: { outline: "outline-hash" },
  manifest: { title: 'Deck', goal: '', audience: '', language: 'zh-CN', pages: '待明确', requirements: [], prohibitions: [] },
  outline,
  design: { demands: [], decorations: { page_number: 'bottom-right', deck_title: 'none', section_title: 'none', key_message: 'none' } },
  slides_by_id: {
    sli_1: { spec_state: 'pending', spec: null, html_state: 'missing', html_hash: '' },
    sli_2: { spec_state: 'pending', spec: null, html_state: 'missing', html_hash: '' },
    sli_3: { spec_state: 'pending', spec: null, html_state: 'missing', html_hash: '' },
    sli_4: { spec_state: 'pending', spec: null, html_state: 'missing', html_hash: '' },
  },
};

describe('canonical outline selectors', () => {
  it('flattens direct and grouped sections as the only page order', () => {
    expect(flattenOutline(outline).map((item) => item.node.id)).toEqual(['sli_1', 'sli_2', 'sli_3', 'sli_4']);
    expect(ordinalBySlideId(outline)).toEqual({ sli_1: 1, sli_2: 2, sli_3: 3, sli_4: 4 });
  });

  it('follows section moves in page order', () => {
    const moved = { ...outline, sections: [outline.sections[1], outline.sections[0]] };
    expect(flattenOutline(moved).map((item) => item.node.id)).toEqual(['sli_3', 'sli_4', 'sli_1', 'sli_2']);
  });

  it('derives pending slides and metadata from the current snapshot', () => {
    expect(orderedSlides(snapshot).map((slide) => [slide.id, slide.html_state])).toEqual([
      ['sli_1', 'missing'], ['sli_2', 'missing'], ['sli_3', 'missing'], ['sli_4', 'missing'],
    ]);
    expect(orderedSlides(snapshot).every(slide => slide.purpose === undefined)).toBe(true);
    const withSpec: ProjectContentSnapshot = { ...snapshot, slides_by_id: {
      ...snapshot.slides_by_id,
      sli_4: { ...snapshot.slides_by_id.sli_4, spec_state: 'ready', spec: { purpose: 'content', content_type: 'example', core: '案例证明观点', elements: [] } },
    } };
    expect(orderedSlides(withSpec).find((slide) => slide.id === 'sli_4')).toMatchObject({ purpose: 'content', content_type: 'example' });
  });
});
