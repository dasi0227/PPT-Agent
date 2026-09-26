import { describe, expect, it } from 'vitest';
import type { ProjectContentSnapshot } from '../../api/types';
import { buildRuntimeFrame } from './runtimeFrame';

const snapshot: ProjectContentSnapshot = {
  project_id: 'p',
  theme: 'editorial-serif',
  appearance: null,
  hashes: { outline: "outline-hash" },
  manifest: { title: 'T', goal: '', audience: '', language: 'zh-CN', requirements: [], prohibitions: [] },
  outline: { sections: [{ id: 'sec', title: '开场', purpose: '', slides: [{ slide_id: 'cover', title: '封面' }, { slide_id: 'body', title: '正文' }], subsections: [] }] },
  design: { direction: '', layout_preferences: [], decorations: { page_number: 'bottom-right', deck_title: 'none', section_title: 'none', key_message: 'none' } },
  slides_by_id: {},
};

describe('runtime frame builder', () => {
  it('derives every page ordinal and the fixed canvas without manifest rendering settings', () => {
    expect(buildRuntimeFrame(snapshot, 'cover')).toMatchObject({ project_id: 'p', slide_id: 'cover', ordinal: 1, total: 2, canvas: { width: 1920, height: 1080, aspect_ratio: '16:9' } });
    expect(buildRuntimeFrame(snapshot, 'body')).toMatchObject({ project_id: 'p', slide_id: 'body', ordinal: 2, total: 2, canvas: { width: 1920, height: 1080, aspect_ratio: '16:9' } });
    expect(buildRuntimeFrame(snapshot, 'body')?.theme_id).toBe('editorial-serif');
    expect(buildRuntimeFrame(snapshot, 'cover')?.role).toBeUndefined();
    const withSpec: ProjectContentSnapshot = { ...snapshot, slides_by_id: {
      cover: { spec_state: 'ready', spec: { role: 'cover', key_message: '开场', elements: [] }, html_state: 'missing', html_hash: '' },
    } };
    expect(buildRuntimeFrame(withSpec, 'cover')).toMatchObject({ role: 'cover', key_message: '开场' });
  });

  it('recomputes ordinals from reordered outline without touching HTML', () => {
    const reordered = { ...snapshot, outline: { ...snapshot.outline, sections: [{ ...snapshot.outline.sections[0], slides: [...snapshot.outline.sections[0].slides].reverse() }] } };
    expect(buildRuntimeFrame(reordered, 'body')?.ordinal).toBe(1);
    expect(buildRuntimeFrame(reordered, 'cover')?.ordinal).toBe(2);
  });
});
