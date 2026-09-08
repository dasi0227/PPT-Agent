import type { CreateRunRequest, RunMode, ScopeObject, ScopeSelectionKind, Slide } from '../../api/types';

export function createTargetedRun(input: {
  object: ScopeObject;
  selection: ScopeSelectionKind;
  mode: RunMode;
  instruction: string;
  slides: Slide[];
  currentSlideId: string | null;
}): CreateRunRequest {
  const slide = input.slides.find((candidate) => candidate.id === input.currentSlideId);
  const selection = input.object === 'global' || (input.selection === 'current_page' && !slide)
    ? 'all_pages'
    : input.selection;
  return {
    scope: {
      object: input.object,
      selection: {
        kind: selection,
        ...(selection === 'current_page' && slide ? { current_slide_id: slide.id } : {}),
      },
    },
    mode: input.mode,
    instruction: input.instruction,
  };
}
