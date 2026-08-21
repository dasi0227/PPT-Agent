import type { Artifact, CreateRunRequest, RunMode, ScopeLevel, Slide } from '../../api/types';

export function createTargetedRun(input: {
  artifact: Artifact;
  level: ScopeLevel;
  mode: RunMode;
  instruction: string;
  slides: Slide[];
  currentSlideId: string | null;
}): CreateRunRequest {
  const slide = input.slides.find((candidate) => candidate.id === input.currentSlideId);
  const level = input.level === 'slide' && !slide ? 'deck' : input.level;
  return {
    scope: {
      artifact: input.artifact,
      level,
      ...(level === 'slide' && slide ? { slide_id: slide.id } : {}),
    },
    mode: input.mode,
    instruction: input.instruction,
  };
}
