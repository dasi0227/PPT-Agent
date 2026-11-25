import type { Outline, OutlineChapter, OutlineSlideNode, OutlineSection, ProjectContentSnapshot, Slide } from '../../api/types';

export interface FlatOutlineSlide { node: OutlineSlideNode; chapter: OutlineChapter; section?: OutlineSection; ordinal: number }

export function flattenOutline(outline?: Outline): FlatOutlineSlide[] {
  if (!outline) return [];
  const result: FlatOutlineSlide[] = [];
  outline.chapters.forEach((chapter) => {
    chapter.slides.forEach((node) => result.push({ node, chapter, ordinal: result.length + 1 }));
    chapter.sections.forEach((section) => section.slides.forEach((node) => result.push({ node, chapter, section, ordinal: result.length + 1 })));
  });
  return result;
}

export function slideSpecPath(slideId: string): string { return `.${slideId}.json`; }

export function orderedSlides(snapshot?: ProjectContentSnapshot): Slide[] {
  if (!snapshot) return [];
  return flattenOutline(snapshot.outline).map(({ node, chapter, section }) => {
    const content = snapshot.slides_by_id[node.id];
    const spec = content?.spec ?? undefined;
    return {
      id: node.id, project_id: snapshot.project_id,
      title: node.title, purpose: spec?.purpose,
      layout: spec?.layout ?? '', html_path: content?.html_hash ? `${node.id}.html` : '',
      spec_path: spec || content?.spec_state === 'invalid' ? slideSpecPath(node.id) : '', html_hash: content?.html_hash ?? '',
      chapterId: chapter.id, sectionId: section?.id, spec,
      html_state: content?.html_state ?? 'missing',
    };
  });
}

export function hasRenderedHTML(slide: Slide): boolean {
  return slide.html_state === 'available';
}

export function ordinalBySlideId(outline?: Outline): Record<string, number> { return Object.fromEntries(flattenOutline(outline).map((item) => [item.node.id, item.ordinal])); }
