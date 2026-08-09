import type { Artifact, CreateRunRequest, RunIntent, ScopeLevel, Slide } from '../../api/types';

export function createTargetedRun(input: {
  artifact: Artifact;
  level: ScopeLevel;
  intent: RunIntent;
  instruction: string;
  slides: Slide[];
  currentPage: number;
}): CreateRunRequest {
  const slide = input.slides[input.currentPage];
  const level = input.level === 'slide' && !slide ? 'deck' : input.level;
  return {
    scope: {
      artifact: input.artifact,
      level,
      ...(level === 'slide' ? { slide_id: slide.id } : {}),
    },
    intent: input.intent,
    instruction: input.instruction,
  };
}
