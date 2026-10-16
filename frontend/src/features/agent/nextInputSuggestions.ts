import { defaultBindings, matchesShortcut, type ShortcutBindings } from '../../lib/shortcuts';
import type { CommandExecution } from '../../api/commands';
import type { SSEEvent } from '../../api/types';

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

export function reduceNextInputSuggestions(
  state: NextInputSuggestionsState | null,
  event: SSEEvent,
  activeRunId?: string | null,
): NextInputSuggestionsState | null {
  if (activeRunId && event.data.run_id !== activeRunId) return state;
  switch (event.event) {
    case 'run.resumed':
    case 'run.started':
      return null;
    case 'run.failed':
    case 'run.error':
    case 'run.canceled':
      return state?.runId === event.data.run_id ? null : state;
    default:
      return state;
  }
}

export function reduceSuggestCommand(
  state: NextInputSuggestionsState | null,
  command: CommandExecution<{ requests: string[] }>,
  runId: string | null,
  runStatus: string,
): NextInputSuggestionsState | null {
  if (command.kind !== 'suggest' || !runId || command.run_id !== runId || !['done', 'error', 'canceled'].includes(runStatus)) return state;
  if (command.status === 'accepted') {
    return { runId, attemptId: command.attempt_id, items: [], status: 'pending' };
  }
  if (state?.attemptId !== command.attempt_id) return state;
  if (command.status === 'completed') {
    const items = command.result?.requests;
    if (!Array.isArray(items) || items.length > 3 || items.some(item => typeof item !== 'string'
      || !item.trim() || Array.from(item).length > 80 || /[\r\n\t\p{Cc}\p{Cf}]/u.test(item))
      || new Set(items.map(item => item.toLocaleLowerCase())).size !== items.length) return null;
    return { ...state, items, status: 'eligible' };
  }
  if (['failed', 'canceled', 'interrupted'].includes(command.status)) return null;
  return state;
}
