import { describe, expect, it } from 'vitest';
import { defaultBindings, matchesShortcut, recordShortcut, validateBindings } from './shortcuts';
import { findCommandTrigger, findComponentTrigger, findPageTrigger } from '../features/agent/promptMatching';
const event = { code: 'ArrowLeft', metaKey: true, ctrlKey: false, altKey: false, shiftKey: false };
describe('configurable shortcuts', () => {
  it('matches platform modifiers exactly and supports either keyboard spelling of plus', () => {
    expect(matchesShortcut(event, defaultBindings['deck.previous'], true)).toBe(true);
    expect(matchesShortcut({ ...event, metaKey: false }, defaultBindings['deck.previous'], true)).toBe(false);
    expect(matchesShortcut({ ...event, metaKey: false, ctrlKey: true }, defaultBindings['deck.previous'], false)).toBe(true);
    expect(matchesShortcut({ ...event, altKey: true }, defaultBindings['deck.previous'], true)).toBe(false);
    expect(matchesShortcut({ ...event, isComposing: true }, defaultBindings['deck.previous'], true)).toBe(false);
    for (const shiftKey of [true,false]) expect(matchesShortcut({ ...event, code: 'Equal', shiftKey }, defaultBindings['deck.zoom_in'], true)).toBe(true);
    expect(recordShortcut({ ...event, code:'Digit1', key:'¡', metaKey:false, altKey:true }, true).code).toBe('Digit1');
  });
  it('rejects collisions across groups, malformed bindings, and native editing shortcuts', () => {
    expect(validateBindings(defaultBindings)).toBeNull();
    expect(validateBindings({ ...defaultBindings, 'deck.theme': defaultBindings['composer.submit'] })).toContain('冲突');
    expect(validateBindings({ ...defaultBindings, 'trigger.page': { trigger: '/' } })).toContain('冲突');
    expect(validateBindings({ ...defaultBindings, 'trigger.page': { trigger: 'x' } })).not.toBeNull();
    expect(validateBindings({ ...defaultBindings, 'deck.theme': { code:'KeyC', primary:true } })).toContain('保留');
  });
  it('uses literal configurable symbols, including regex metacharacters, without keeping the old trigger', () => {
    expect(findCommandTrigger('!theme', 6, '!')?.query).toBe('theme');
    expect(findCommandTrigger('/theme', 6, '!')).toBeNull();
    expect(findPageTrigger('^page', 5, '^')?.query).toBe('page');
    expect(findComponentTrigger('$card', 5, '.')).toBeNull();
    expect(findComponentTrigger('.card', 5, '.')?.query).toBe('card');
  });
});
