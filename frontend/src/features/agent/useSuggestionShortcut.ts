import { useEffect, useLayoutEffect, useRef, useState } from 'react';
import { eventCode, shortcutOverlayOpen } from '../../lib/shortcuts';
import { useShortcutStore } from '../../stores/shortcutStore';
import { nextInputShortcutIndex } from './nextInputSuggestions';

interface SuggestionShortcutOptions {
  visible: boolean;
  suggestionKey: string | null;
  items: readonly string[];
  blocked: boolean;
  isComposing: boolean;
  onSelect: (value: string) => void;
}

interface PressedSuggestion {
  code: string;
  index: number;
  value: string;
  suggestionKey: string | null;
}

export function useSuggestionShortcut(options: SuggestionShortcutOptions): number | null {
  const latest = useRef(options);
  const pending = useRef<PressedSuggestion | null>(null);
  const heldCodes = useRef(new Set<string>());
  const [pressedIndex, setPressedIndex] = useState<number | null>(null);

  useLayoutEffect(() => {
    latest.current = options;
    const pressed = pending.current;
    if (pressed && (!options.visible || options.blocked || options.isComposing
      || pressed.suggestionKey !== options.suggestionKey || options.items[pressed.index] !== pressed.value)) {
      pending.current = null;
      setPressedIndex(null);
    }
  });

  useEffect(() => {
    const cancel = () => {
      if (!pending.current) return;
      pending.current = null;
      setPressedIndex(null);
    };
    const reset = () => { heldCodes.current.clear(); cancel(); };
    const inputFor = (event: KeyboardEvent) => {
      const current = latest.current;
      const target = event.target instanceof Element ? event.target : null;
      const shortcuts = useShortcutStore.getState();
      return {
        altKey: event.altKey, metaKey: event.metaKey, ctrlKey: event.ctrlKey, shiftKey: event.shiftKey,
        code: eventCode(event),
        isComposing: event.isComposing || current.isComposing,
        blocked: !current.visible || current.blocked || !shortcuts.ready || shortcutOverlayOpen()
          || Boolean(document.querySelector('[role="grid"]')),
        targetEditable: Boolean(target?.closest('input, textarea, select, [contenteditable="true"]')),
        targetIsComposer: Boolean(target?.closest('.composer-prompt-editor')),
      };
    };
    const onKeyDown = (event: KeyboardEvent) => {
      const code = eventCode(event);
      // Consume every repeat until release, even if the suggestion has since disappeared.
      if (heldCodes.current.has(code)) { event.preventDefault(); return; }
      if (event.defaultPrevented) { cancel(); return; }
      const index = nextInputShortcutIndex(inputFor(event), useShortcutStore.getState().bindings);
      const current = latest.current;
      if (index === null || !current.items[index]) { cancel(); return; }
      event.preventDefault();
      if (event.repeat) return;
      heldCodes.current.add(code);
      pending.current = { code, index, value: current.items[index], suggestionKey: current.suggestionKey };
      setPressedIndex(index);
    };
    const onKeyUp = (event: KeyboardEvent) => {
      const code = eventCode(event);
      if (!heldCodes.current.delete(code)) return;
      event.preventDefault();
      const pressed = pending.current;
      if (!pressed || pressed.code !== code) return;
      cancel();
      const current = latest.current;
      const input = inputFor(event);
      if (input.blocked || input.isComposing || (input.targetEditable && !input.targetIsComposer)
        || pressed.suggestionKey !== current.suggestionKey || current.items[pressed.index] !== pressed.value) return;
      current.onSelect(pressed.value);
    };
    const onVisibilityChange = () => { if (document.hidden) reset(); };
    window.addEventListener('keydown', onKeyDown, true);
    window.addEventListener('keyup', onKeyUp, true);
    window.addEventListener('blur', reset);
    document.addEventListener('visibilitychange', onVisibilityChange);
    return () => {
      window.removeEventListener('keydown', onKeyDown, true);
      window.removeEventListener('keyup', onKeyUp, true);
      window.removeEventListener('blur', reset);
      document.removeEventListener('visibilitychange', onVisibilityChange);
      heldCodes.current.clear();
      pending.current = null;
    };
  }, []);

  return options.visible ? pressedIndex : null;
}
