import type { Decorations, ProjectContentSnapshot, SlideContentType, SlidePurpose } from '../../api/types';
import { flattenOutline } from '../deck/selectors';

export interface RuntimeFrameContext {
	project_id?: string;
  slide_id: string;
  canvas: { width: 1920; height: 1080; aspect_ratio: '16:9' };
  theme_id: string;
  appearance: import("../../api/types").RuntimeAppearance | null;
  deck_title: string;
  ordinal: number;
  total: number;
  purpose?: SlidePurpose;
  content_type?: SlideContentType;
  section: { id: string; title: string; index: number };
  subsection?: { id: string; title: string; index: number };
  decorations: Decorations;
  key_message: string;
}

export function buildRuntimeFrame(snapshot: ProjectContentSnapshot, slideId: string): RuntimeFrameContext | undefined {
  const flat = flattenOutline(snapshot.outline);
  const item = flat.find((candidate) => candidate.node.slide_id === slideId);
  if (!item) return undefined;
  const sectionIndex = snapshot.outline.sections.findIndex((section) => section.id === item.section.id);
  const subsectionIndex = item.subsection ? item.section.subsections.findIndex((subsection) => subsection.id === item.subsection?.id) : -1;
  return {
	project_id: snapshot.project_id,
	slide_id: slideId,
    canvas: { width: 1920, height: 1080, aspect_ratio: '16:9' },
    theme_id: snapshot.theme,
    appearance: snapshot.appearance,
    deck_title: snapshot.manifest.title,
    ordinal: item.ordinal,
    total: flat.length,
    purpose: snapshot.slides_by_id[slideId]?.spec?.purpose,
    content_type: snapshot.slides_by_id[slideId]?.spec?.content_type,
    section: { id: item.section.id, title: item.section.title, index: sectionIndex + 1 },
    ...(item.subsection ? { subsection: { id: item.subsection.id, title: item.subsection.title, index: subsectionIndex + 1 } } : {}),
    decorations: { ...snapshot.design.decorations },
    key_message: snapshot.slides_by_id[slideId]?.spec?.core ?? '',
  };
}
