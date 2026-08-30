import type { PublicTarget } from '../../api/types';

const deckFileNames: Partial<Record<PublicTarget['part'], string>> = {
  manifest: 'manifest.json',
  outline: 'outline.json',
  design: 'design.json',
};

const slideFileNames: Partial<Record<PublicTarget['part'], string>> = {
  spec: 'spec.json',
  html: 'index.html',
};

export function targetFileLabel(target: PublicTarget | undefined): string | undefined {
  if (!target) return undefined;

  if (target.type === 'deck') return deckFileNames[target.part];

  if (target.type === 'slide' && target.slide_id) {
    const fileName = slideFileNames[target.part];
    if (fileName) return `${target.slide_id}/${fileName}`;
  }

  return undefined;
}
