import { defaultBindings, matchesShortcut, type ShortcutBindings } from '../../lib/shortcuts';
import type { SSEEvent } from '../../api/types';

export interface NextInputSuggestionsState {
  runId: string;
  messageId: string;
  items: string[];
  status: 'staged' | 'eligible';
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

export function reduceNextInputSuggestions(
  state: NextInputSuggestionsState | null,
  event: SSEEvent,
  activeRunId?: string | null,
): NextInputSuggestionsState | null {
  if (activeRunId && event.data.run_id !== activeRunId) return state;
  switch (event.event) {
    case 'run.started':
      return null;
    case 'message.final':
      return event.data.suggested_next_inputs.length > 0
        ? {
            runId: event.data.run_id,
            messageId: event.data.message_id,
            items: event.data.suggested_next_inputs,
            status: 'staged',
          }
        : null;
    case 'run.completed':
      return state?.runId === event.data.run_id ? { ...state, status: 'eligible' } : state;
    case 'run.failed':
    case 'run.error':
    case 'run.canceled':
      return state?.runId === event.data.run_id ? null : state;
    default:
      return state;
  }
}
