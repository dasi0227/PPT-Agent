import { useEffect, useRef } from 'react';
import { useShortcutStore } from '../stores/shortcutStore';
import { fixedBindings, matchesShortcut, shortcutOverlayOpen } from './shortcuts';

/** Protect text input; the source editor also allows switching its view and form. */
export function useAppShortcuts(actions: Record<string, () => void>) {
  const latest = useRef(actions);
  latest.current = actions;
  useEffect(() => {
    const keydown = (event: KeyboardEvent) => {
      const state = useShortcutStore.getState();
      if (event.defaultPrevented || event.isComposing || shortcutOverlayOpen()) return;
      const target = event.target instanceof Element ? event.target : null;
      // A focused slider owns its unmodified navigation and activation keys.
      if (target?.closest('[role="slider"]') && !event.metaKey && !event.ctrlKey && !event.altKey && !event.shiftKey
        && ['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown', 'Home', 'End', ' ', 'Enter'].includes(event.key)) return;
      const editing = target?.closest('input, textarea, select, [contenteditable="true"], [role="textbox"]');
      for (const [id, run] of Object.entries(latest.current)) {
        if (editing && !(target?.closest('.cm-content') && (id === 'deck.view' || id === 'deck.form'))) continue;
        const binding = fixedBindings[id] ?? (state.ready ? state.bindings[id] : undefined);
        if (!matchesShortcut(event, binding)) continue;
        event.preventDefault();
        event.stopPropagation();
        if (!event.repeat) run();
        return;
      }
    };
    const forwarded = (event: Event) => {
      const id: unknown = (event as CustomEvent).detail;
      if (!useShortcutStore.getState().ready || shortcutOverlayOpen() || typeof id !== 'string' || !id.startsWith('deck.')) return;
      latest.current[id]?.();
    };
    window.addEventListener('ppt-deck-shortcut', forwarded);
    window.addEventListener('keydown', keydown, true);
    return () => { window.removeEventListener('keydown', keydown, true); window.removeEventListener('ppt-deck-shortcut', forwarded); };
  }, []);
}
