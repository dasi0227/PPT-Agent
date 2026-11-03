import { defaultBindings, matchesShortcut, type ShortcutBindings } from '../../lib/shortcuts';

export interface NextInputSuggestionsState {
  runId: string;
  attemptId: string;
  items: string[];
  status: 'pending' | 'eligible';
}

export interface NextInputShortcutInput {
  altKey: boolean;
  metaKey: boolean;
  ctrlKey: boolean;
  shiftKey: boolean;
  isComposing: boolean;
  code: string;
  blocked: boolean;
  targetEditable: boolean;
  targetIsComposer: boolean;
}

export function nextInputShortcutIndex(input: NextInputShortcutInput, bindings: ShortcutBindings = defaultBindings): number | null {
  if (input.isComposing || input.blocked) return null;
  if (input.targetEditable && !input.targetIsComposer) return null;
  const index = [1, 2, 3].findIndex(number => matchesShortcut(input, bindings[`composer.suggestion${number}`]));
  return index < 0 ? null : index;
}
