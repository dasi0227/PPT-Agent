import { describe, expect, it } from 'vitest';
import type { ProjectContentSnapshot } from '../../api/types';
import { buildRuntimeFrame } from './runtimeFrame';

const snapshot: ProjectContentSnapshot = {
  deck: { version: '4.0', revision: 1, project_id: 'p', title: 'T', goal: '', audience: '', language: 'zh-CN', requirements: [], prohibitions: [], canvas: { aspect_ratio: '16:9' }, numbering: { enabled: true, hidden_roles: ['cover'], format: 'number' }, created_at: 1, updated_at: 1 },
  outline: { version: '4.0', revision: 1, project_id: 'p', created_at: 1, updated_at: 1, sections: [{ id: 'sec', title: '开场', purpose: '', slides: [{ slide_id: 'cover', label: '封面', role: 'cover' }, { slide_id: 'body', label: '正文', role: 'content' }], subsections: [] }] },
  design: { version: '4.0', revision: 1, project_id: 'p', theme: '', direction: '', density: 'medium', chrome: [{ type: 'page_number', placement: 'bottom-right', style: 'tiny muted mono' }], created_at: 1, updated_at: 1 },
  slides_by_id: {},
};

describe('runtime frame builder', () => {
  it('hides the cover number while preserving the second page ordinal', () => {
    expect(buildRuntimeFrame(snapshot, 'cover')?.numbering.visible).toBe(false);
    expect(buildRuntimeFrame(snapshot, 'body')).toMatchObject({ ordinal: 2, total: 2, numbering: { visible: true } });
  });

  it('recomputes ordinals from reordered outline without touching HTML', () => {
    const reordered = { ...snapshot, outline: { ...snapshot.outline, sections: [{ ...snapshot.outline.sections[0], slides: [...snapshot.outline.sections[0].slides].reverse() }] } };
    expect(buildRuntimeFrame(reordered, 'body')?.ordinal).toBe(1);
    expect(buildRuntimeFrame(reordered, 'cover')?.ordinal).toBe(2);
  });
});
