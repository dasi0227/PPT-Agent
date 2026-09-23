import type { CreateRunRequest, RunMode, ScopeSelectionKind, Slide } from '../../api/types';

export function createTargetedRun(input: {
  selection: ScopeSelectionKind;
  mode: RunMode;
  instruction: string;
  slides: Slide[];
  currentSlideId: string | null;
}): CreateRunRequest {
  const slide = input.slides.find((candidate) => candidate.id === input.currentSlideId);
  const selection = (input.selection === 'current_page' && !slide)
    ? 'all_pages'
    : input.selection;
  return {
    scope: {
      selection: {
        kind: selection,
        ...(selection === 'current_page' && slide ? { current_slide_id: slide.id } : {}),
      },
    },
    mode: input.mode,
    instruction: input.instruction,
  };
}
