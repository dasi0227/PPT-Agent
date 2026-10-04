import type { DecorationPlacement, Decorations, DecorationType } from '../../api/types';

export const decorationTypes: DecorationType[] = ['page_number', 'section_title', 'deck_title', 'key_message'];
export const decorationPlacements: (DecorationPlacement | 'none')[] = [
  'none', 'top-left', 'top-center', 'top-right', 'bottom-left', 'bottom-center', 'bottom-right',
];

export function decorationPositionOccupants(value: Decorations, type: DecorationType, placement: string): DecorationType[] {
  return placement === 'none' ? [] : decorationTypes.filter(key => key !== type && value[key] === placement);
}

export function hasDecorationPositionConflict(value: Decorations): boolean {
  const positions = decorationTypes.map(key => value[key]).filter(position => position !== 'none');
  return new Set(positions).size !== positions.length;
}
