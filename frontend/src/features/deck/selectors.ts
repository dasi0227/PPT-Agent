import type { Outline, OutlineSection, OutlineSlideNode, OutlineSubsection, ProjectContentSnapshot, Slide } from '../../api/types';

export interface FlatOutlineSlide { node: OutlineSlideNode; section: OutlineSection; subsection?: OutlineSubsection; ordinal: number }

export function flattenOutline(outline?: Outline): FlatOutlineSlide[] {
  if (!outline) return [];
  const result: FlatOutlineSlide[] = [];
  outline.sections.forEach((section) => {
    section.slides.forEach((node) => result.push({ node, section, ordinal: result.length + 1 }));
    section.subsections.forEach((subsection) => subsection.slides.forEach((node) => result.push({ node, section, subsection, ordinal: result.length + 1 })));
  });
  return result;
}

export function orderedSlides(snapshot?: ProjectContentSnapshot): Slide[] {
  if (!snapshot) return [];
  return flattenOutline(snapshot.outline).map(({ node, section, subsection }) => {
    const content = snapshot.slides_by_id[node.slide_id];
    const spec = content?.spec ?? undefined;
    return {
      id: node.slide_id, project_id: snapshot.outline.project_id,
      title: node.title, role: node.role,
      layout: spec?.layout ?? '', html_path: content?.html_revision ? `slides/${node.slide_id}/index.html` : '',
      spec_path: spec ? `slides/${node.slide_id}/spec.json` : '', current_version: content?.html_revision ?? 0,
      spec_revision: spec?.revision ?? 0, html_revision: content?.html_revision ?? 0,
      sectionId: section.id, subsectionId: subsection?.id, spec,
      materialization: { state: content?.html_state ?? 'not_materialized', revisions: { slide_html: content?.html_revision ?? 0, source_outline: snapshot.outline.revision, source_spec: content?.materialization?.source.spec_revision ?? 0, source_design: content?.materialization?.source.design_revision ?? 0 } },
    };
  });
}

export function ordinalBySlideId(outline?: Outline): Record<string, number> { return Object.fromEntries(flattenOutline(outline).map((item) => [item.node.slide_id, item.ordinal])); }
export function selectedSlide(snapshot: ProjectContentSnapshot | undefined, currentSlideId: string | null): Slide | undefined { const slides = orderedSlides(snapshot); return slides.find((slide) => slide.id === currentSlideId) ?? slides[0]; }
export function adjacentSlideIds(outline: Outline | undefined, currentSlideId: string | null): { previous?: string; next?: string } { const flat = flattenOutline(outline); const index = flat.findIndex((item) => item.node.slide_id === currentSlideId); return { previous: index > 0 ? flat[index - 1].node.slide_id : undefined, next: index >= 0 && index + 1 < flat.length ? flat[index + 1].node.slide_id : undefined }; }
