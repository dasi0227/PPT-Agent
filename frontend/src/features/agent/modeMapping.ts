import type { Artifact, CreateRunRequest, InteractionIntent, Slide, TargetLevel } from '../../api/types';

export function createTargetedRun(input: {
  artifact: Artifact;
  level: TargetLevel;
  intent: InteractionIntent;
  instruction: string;
  slides: Slide[];
  currentPage: number;
}): CreateRunRequest {
  const slide = input.slides[input.currentPage];
  const level = input.level === 'slide' && !slide ? 'deck' : input.level;
  return {
    target: {
      artifact: input.artifact,
      level,
      ...(level === 'slide' ? { slide_id: slide.id } : {}),
    },
    interaction: {
      intent: input.intent,
    },
    instruction: input.instruction,
  };
}
