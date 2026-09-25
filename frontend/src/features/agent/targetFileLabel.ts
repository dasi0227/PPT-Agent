import type { PublicTarget } from '../../api/types';
import { partLabel } from '../viewer/semanticLabels';

export function isAuthoringDataTarget(target: PublicTarget | undefined): boolean {
  return Boolean(target && ['manifest', 'outline', 'design', 'spec'].includes(target.part));
}

export function targetFileLabel(target: PublicTarget | undefined, pageName?: string): string | undefined {
  if (!target || target.type === 'file') return undefined;
  const part = partLabel(target.part);
  if (target.type === 'deck') return part;
  if (target.type === 'slide') return `${pageName || target.display_name || '相关页面'} · ${part}`;
  return undefined;
}
