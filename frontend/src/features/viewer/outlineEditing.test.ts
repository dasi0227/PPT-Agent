import { describe, expect, it } from 'vitest';
import type { Outline, OutlineSubsection } from '../../api/types';
import { applyOutlineCommand, outlineDestinations, outlineMutation } from './outlineEditing';

function outline(): Outline {
  return { sections: [
    { id: 'sec_a', title: '开场', purpose: '说明主题', slides: [{ id: 'sli_a', title: '封面' }, { id: 'sli_b', title: '目标' }], subsections: [] },
    { id: 'sec_b', title: '方法', purpose: '组织内容', slides: [], subsections: [{ id: 'sub_b', title: '基础', purpose: '说明方法', slides: [] }] },
  ] };
}

describe('outline draft operations', () => {
  it('keeps a provisional first subsection isolated, including its relocated pages', () => {
    const base = outline();
    const node: OutlineSubsection = { id: 'sub_a', title: '导言', purpose: '展开主题', slides: [] };
    const command = { type: 'insert' as const, kind: 'subsection' as const, node, position: { parent_id: 'sec_a' } };
    const preview = applyOutlineCommand(base, command);
    expect(preview.sections[0].slides).toEqual([]);
    expect(preview.sections[0].subsections[0].slides.map(page => page.id)).toEqual(['sli_a', 'sli_b']);
    expect(base.sections[0].slides).toHaveLength(2);
    expect(base.sections[0].subsections).toEqual([]);
    expect(node.slides).toEqual([]);
    expect(outlineMutation(command)).toMatchObject({ op: 'outline.insert', direct_slides_policy: 'move_into_new_subsection', node: { client_ref: 'sub_a' } });
  });

  it('preserves page IDs and titles when moving across parents or reordering', () => {
    const base = outline();
    const reordered = applyOutlineCommand(base, { type: 'move', id: 'sli_a', position: { parent_id: 'sec_a', after_id: 'sli_b' } });
    expect(reordered.sections[0].slides.map(page => page.id)).toEqual(['sli_b', 'sli_a']);
    const relocated = applyOutlineCommand(reordered, { type: 'move', id: 'sli_a', position: { parent_id: 'sub_b' } });
    expect(relocated.sections[0].slides).toEqual([{ id: 'sli_b', title: '目标' }]);
    expect(relocated.sections[1].subsections[0].slides).toEqual([{ id: 'sli_a', title: '封面' }]);
    expect(base.sections[0].slides).toHaveLength(2);
  });

  it('rejects mixing direct pages and subsections without removing the original node', () => {
    const base = outline();
    expect(() => applyOutlineCommand(base, { type: 'move', id: 'sub_b', position: { parent_id: 'sec_a' } })).toThrow();
    expect(() => applyOutlineCommand(base, { type: 'move', id: 'sli_a', position: { parent_id: 'sec_b' } })).toThrow();
    expect(base.sections[1].subsections).toHaveLength(1);
    expect(base.sections[0].slides).toHaveLength(2);
    expect(outlineDestinations(base, 'slide', 'sec_a')).toEqual([{ id: 'sub_b', title: '方法 / 基础' }]);
    expect(outlineDestinations(base, 'subsection', 'sec_b')).toEqual([]);
  });

  it('promotes pages when removing the last grouping and deletes draft subtrees together', () => {
    const base = outline();
    base.sections[1].subsections[0].slides = [{ id: 'sli_c', title: '概念' }];
    const promoted = applyOutlineCommand(base, { type: 'remove', id: 'sub_b', promote: true });
    expect(promoted.sections[1].subsections).toEqual([]);
    expect(promoted.sections[1].slides).toEqual([{ id: 'sli_c', title: '概念' }]);
    const deleted = applyOutlineCommand(base, { type: 'remove', id: 'sec_b' });
    expect(deleted.sections.map(section => section.id)).toEqual(['sec_a']);
    expect(base.sections[1].subsections[0].slides).toHaveLength(1);
  });

  it('rejects stale position anchors and destination capacity without mutating the draft', () => {
    const base = outline();
    expect(() => applyOutlineCommand(base, { type: 'move', id: 'sli_a', position: { parent_id: 'sec_a', before_id: 'missing' } })).toThrow();
    base.sections[1].subsections[0].slides = Array.from({ length: 200 }, (_, index) => ({ id: `sli_${index}`, title: '内容' }));
    expect(() => applyOutlineCommand(base, { type: 'move', id: 'sli_a', position: { parent_id: 'sub_b' } })).toThrow();
    expect(base.sections[0].slides).toHaveLength(2);
    expect(outlineDestinations(base, 'slide', 'sec_a')).toEqual([]);
  });
});
