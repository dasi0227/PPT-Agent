import type { MutationPosition, Outline, OutlineSection, OutlineSlideNode, OutlineSubsection, PPTMutation } from '../../api/types';

export type OutlineKind = 'section' | 'subsection' | 'slide';
export type OutlineNode = OutlineSection | OutlineSubsection | OutlineSlideNode;
export type OutlineCommand =
  | { type: 'insert'; kind: OutlineKind; node: OutlineNode; position: MutationPosition }
  | { type: 'update'; id: string; title: string; purpose?: string }
  | { type: 'move'; id: string; position: MutationPosition }
  | { type: 'remove'; id: string; promote?: boolean };

export function findOutlineNode(outline: Outline, id: string) {
  for (const section of outline.sections) {
    if (section.id === id) return { node: section, kind: 'section' as const, parent: undefined, siblings: outline.sections as OutlineNode[] };
    for (const slide of section.slides) {
      if (slide.id === id) return { node: slide, kind: 'slide' as const, parent: section, siblings: section.slides as OutlineNode[] };
    }
    for (const sub of section.subsections) {
      if (sub.id === id) return { node: sub, kind: 'subsection' as const, parent: section, siblings: section.subsections as OutlineNode[] };
      for (const slide of sub.slides) {
        if (slide.id === id) return { node: slide, kind: 'slide' as const, parent: sub, siblings: sub.slides as OutlineNode[] };
      }
    }
  }
}

export function outlineDestinations(outline: Outline, kind: OutlineKind, parentId?: string) {
  return outline.sections.flatMap(section => {
    if (kind === 'section') return [];
    if (kind === 'subsection') return section.id !== parentId && !section.slides.length && section.subsections.length < 64
      ? [{ id: section.id, title: section.title }] : [];
    return section.subsections.length
      ? section.subsections.filter(sub => sub.id !== parentId && sub.slides.length < 200).map(sub => ({ id: sub.id, title: `${section.title} / ${sub.title}` }))
      : section.id !== parentId && section.slides.length < 200 ? [{ id: section.id, title: section.title }] : [];
  });
}

export function newOutlineNode(kind: OutlineKind): OutlineNode {
  const prefix = { section: 'sec', subsection: 'sub', slide: 'sli' }[kind];
  const node = { id: `${prefix}_${crypto.randomUUID().replace(/-/g, '')}`, title: { section: '新章节', subsection: '新小节', slide: '新页面' }[kind] };
  if (kind === 'slide') return node;
  const container = { ...node, purpose: '待明确', slides: [] };
  return kind === 'section' ? { ...container, subsections: [] } : container;
}

/** Draft edits preserve existing IDs and never mutate the committed outline. */
export function applyOutlineCommand(outline: Outline, command: OutlineCommand): Outline {
  const next = structuredClone(outline);
  if (command.type === 'update') {
    const item = findOutlineNode(next, command.id);
    if (!item) throw new Error('目录节点已更新');
    item.node.title = command.title;
    if ('purpose' in item.node && command.purpose !== undefined) item.node.purpose = command.purpose;
    return next;
  }
  if (command.type === 'remove') {
    const item = findOutlineNode(next, command.id);
    if (!item) throw new Error('目录节点已更新');
    if (command.promote && item.kind === 'subsection' && item.parent && 'subsections' in item.parent) {
      if (item.parent.subsections.length !== 1) throw new Error('仅最后一个小节可以取消分组');
      item.parent.slides = (item.node as OutlineSubsection).slides;
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
  if (kind === 'section') siblings = next.sections;
  else if (kind === 'subsection' && parent?.kind === 'section') {
    const section = parent.node as OutlineSection;
    if (section.slides.length) {
      if (command.type !== 'insert') throw new Error('不能移动小节到含有直接页面的章节');
      (node as OutlineSubsection).slides = section.slides;
      section.slides = [];
    }
    siblings = section.subsections;
  } else if (kind === 'slide' && parent && 'slides' in parent.node && !('subsections' in parent.node && parent.node.subsections.length)) {
    siblings = parent.node.slides;
  } else throw new Error('移动位置无效');
  const limit = kind === 'section' ? 32 : kind === 'subsection' ? 64 : 200;
  if (siblings.length >= limit) throw new Error('节点数量已达上限');
  const { before_id, after_id } = command.position;
  const anchor = before_id ?? after_id;
  const index = anchor ? siblings.findIndex(value => value.id === anchor) : siblings.length;
  if (index < 0) throw new Error('移动位置已更新');
  siblings.splice(index + (after_id ? 1 : 0), 0, node);
  return next;
}

export function outlineMutation(command: OutlineCommand): PPTMutation {
  if (command.type === 'update') return { op: 'outline.update', node_id: command.id,
    changes: { title: command.title, ...(command.purpose !== undefined ? { purpose: command.purpose } : {}) } };
  if (command.type === 'remove') return { op: 'outline.remove', node_id: command.id, ...(command.promote ? { child_policy: 'promote_to_section' } : {}) };
  if (command.type === 'move') return { op: 'outline.move', node_id: command.id, position: command.position };
  const { node, kind } = command;
  const common = { client_ref: node.id, title: node.title };
  return { op: 'outline.insert', position: command.position,
    ...(kind === 'subsection' ? { direct_slides_policy: 'move_into_new_subsection' as const } : {}),
    node: kind === 'section' ? { kind, ...common, purpose: (node as OutlineSection).purpose, slides: [], subsections: [] }
      : kind === 'subsection' ? { kind, ...common, purpose: (node as OutlineSubsection).purpose } : { kind, ...common } };
}
