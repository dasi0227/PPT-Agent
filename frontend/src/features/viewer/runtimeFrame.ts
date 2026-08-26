import type { ProjectContentSnapshot } from '../../api/types';
import { flattenOutline } from '../deck/selectors';

export interface RuntimeFrameContext {
  slide_id: string;
  deck_title: string;
  ordinal: number;
  total: number;
  role: string;
  section: { id: string; title: string; index: number };
  subsection?: { id: string; title: string; index: number };
  numbering: { visible: boolean; format: 'number' };
  chrome: Array<{ type: 'page_number' | 'section_marker' | 'key_message' | 'deck_title'; placement: string; style: string }>;
}

export function buildRuntimeFrame(snapshot: ProjectContentSnapshot, slideId: string): RuntimeFrameContext | undefined {
  const flat = flattenOutline(snapshot.outline);
  const item = flat.find((candidate) => candidate.node.slide_id === slideId);
  if (!item) return undefined;
  const sectionIndex = snapshot.outline.sections.findIndex((section) => section.id === item.section.id);
  const subsectionIndex = item.subsection ? item.section.subsections.findIndex((subsection) => subsection.id === item.subsection?.id) : -1;
  return {
    slide_id: slideId,
    deck_title: snapshot.deck.title,
    ordinal: item.ordinal,
    total: flat.length,
    role: item.node.role,
    section: { id: item.section.id, title: item.section.title, index: sectionIndex + 1 },
    ...(item.subsection ? { subsection: { id: item.subsection.id, title: item.subsection.title, index: subsectionIndex + 1 } } : {}),
    numbering: {
      visible: snapshot.deck.numbering.enabled && !snapshot.deck.numbering.hidden_roles.includes(item.node.role),
      format: snapshot.deck.numbering.format,
    },
    chrome: snapshot.design.chrome.map((entry) => ({ ...entry })),
  };
}
