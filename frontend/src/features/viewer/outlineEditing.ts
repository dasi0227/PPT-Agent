import type { MutationPosition, Outline, OutlineChapter, OutlineSlideNode, OutlineSection } from '../../api/types';

export type OutlineKind = 'chapter' | 'section' | 'slide';
export type OutlineNode = OutlineChapter | OutlineSection | OutlineSlideNode;
export type OutlineCommand =
  | { type: 'insert'; kind: OutlineKind; node: OutlineNode; position: MutationPosition }
  | { type: 'update'; id: string; title: string }
  | { type: 'move'; id: string; position: MutationPosition }
  | { type: 'remove'; id: string; promote?: boolean };

export function findOutlineNode(outline: Outline, id: string) {
  for (const chapter of outline.chapters) {
    if (chapter.id === id) return { node: chapter, kind: 'chapter' as const, parent: undefined, siblings: outline.chapters as OutlineNode[] };
    for (const slide of chapter.slides) {
      if (slide.id === id) return { node: slide, kind: 'slide' as const, parent: chapter, siblings: chapter.slides as OutlineNode[] };
    }
    for (const sub of chapter.sections) {
      if (sub.id === id) return { node: sub, kind: 'section' as const, parent: chapter, siblings: chapter.sections as OutlineNode[] };
      for (const slide of sub.slides) {
        if (slide.id === id) return { node: slide, kind: 'slide' as const, parent: sub, siblings: sub.slides as OutlineNode[] };
      }
    }
  }
}

export function outlineDestinations(outline: Outline, kind: OutlineKind, parentId?: string) {
  return outline.chapters.flatMap(chapter => {
    if (kind === 'chapter') return [];
    if (kind === 'section') return chapter.id !== parentId && !chapter.slides.length && chapter.sections.length < 64
      ? [{ id: chapter.id, title: chapter.title }] : [];
    return chapter.sections.length
      ? chapter.sections.filter(sub => sub.id !== parentId && sub.slides.length < 200).map(sub => ({ id: sub.id, title: `${chapter.title} / ${sub.title}` }))
      : chapter.id !== parentId && chapter.slides.length < 200 ? [{ id: chapter.id, title: chapter.title }] : [];
  });
}

export function newOutlineNode(kind: OutlineKind): OutlineNode {
  const prefix = kind === 'slide' ? 'sli' : 'sec';
  const node = { id: `${prefix}_${crypto.randomUUID().replace(/-/g, '')}`, title: { chapter: '新章节', section: '新小节', slide: '新页面' }[kind] };
  if (kind === 'slide') return node;
  const container = { ...node, slides: [] };
  return kind === 'chapter' ? { ...container, sections: [] } : container;
}

/** Draft edits preserve existing IDs and never mutate the committed outline. */
export function applyOutlineCommand(outline: Outline, command: OutlineCommand): Outline {
  const next = structuredClone(outline);
  if (command.type === 'update') {
    const item = findOutlineNode(next, command.id);
    if (!item) throw new Error('目录节点已更新');
    item.node.title = command.title;
    return next;
  }
  if (command.type === 'remove') {
    const item = findOutlineNode(next, command.id);
    if (!item) throw new Error('目录节点已更新');
    if (command.promote && item.kind === 'section' && item.parent && 'sections' in item.parent) {
      if (item.parent.sections.length !== 1) throw new Error('仅最后一个小节可以取消分组');
      item.parent.slides = (item.node as OutlineSection).slides;
    }
    item.siblings.splice(item.siblings.findIndex(node => node.id === command.id), 1);
    return next;
  }
  let node: OutlineNode;
  let kind: OutlineKind;
  if (command.type === 'move') {
    const item = findOutlineNode(next, command.id);
    if (!item) throw new Error('目录节点已更新');
    node = item.node; kind = item.kind;
    item.siblings.splice(item.siblings.findIndex(value => value.id === command.id), 1);
  } else {
    node = structuredClone(command.node); kind = command.kind;
  }
  const parent = command.position.parent_id ? findOutlineNode(next, command.position.parent_id) : undefined;
  let siblings: OutlineNode[];
  if (kind === 'chapter') siblings = next.chapters;
  else if (kind === 'section' && parent?.kind === 'chapter') {
    const chapter = parent.node as OutlineChapter;
    if (chapter.slides.length) {
      if (command.type !== 'insert') throw new Error('不能移动小节到含有直接页面的章节');
      (node as OutlineSection).slides = chapter.slides;
      chapter.slides = [];
    }
    siblings = chapter.sections;
  } else if (kind === 'slide' && parent && 'slides' in parent.node && !('sections' in parent.node && parent.node.sections.length)) {
    siblings = parent.node.slides;
  } else throw new Error('移动位置无效');
  const limit = kind === 'chapter' ? 32 : kind === 'section' ? 64 : 200;
  if (siblings.length >= limit) throw new Error('节点数量已达上限');
  const { before_id, after_id } = command.position;
  const anchor = before_id ?? after_id;
  const index = anchor ? siblings.findIndex(value => value.id === anchor) : siblings.length;
  if (index < 0) throw new Error('移动位置已更新');
  siblings.splice(index + (after_id ? 1 : 0), 0, node);
  return next;
}
