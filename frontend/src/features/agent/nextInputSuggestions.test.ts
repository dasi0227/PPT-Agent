import { describe, expect, it } from 'vitest';
import { nextInputShortcutIndex } from './nextInputSuggestions';

describe('next input shortcuts', () => {
  it('only accepts unmodified Alt/Option digit shortcuts outside conflicting UI', () => {
    const valid = {
      altKey: true, metaKey: false, ctrlKey: false, shiftKey: false,
      isComposing: false, code: 'Digit2', blocked: false,
      targetEditable: false, targetIsComposer: false,
    };
    expect(nextInputShortcutIndex(valid)).toBe(1);
    expect(nextInputShortcutIndex({ ...valid, isComposing: true })).toBeNull();
    expect(nextInputShortcutIndex({ ...valid, blocked: true })).toBeNull();
    expect(nextInputShortcutIndex({ ...valid, targetEditable: true })).toBeNull();
    expect(nextInputShortcutIndex({ ...valid, targetEditable: true, targetIsComposer: true })).toBe(1);
    expect(nextInputShortcutIndex({ ...valid, metaKey: true })).toBeNull();
    expect(nextInputShortcutIndex({ ...valid, code: 'Numpad2' })).toBeNull();
  });
});
