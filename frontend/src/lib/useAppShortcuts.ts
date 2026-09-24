import { useEffect, useRef } from 'react';
import { useShortcutStore } from '../stores/shortcutStore';
import { matchesShortcut, shortcutOverlayOpen } from './shortcuts';

/** Application actions only run outside text editing and modal/menu interactions. */
export function useAppShortcuts(actions: Record<string, () => void>) {
  const latest = useRef(actions);
  latest.current = actions;
  useEffect(() => {
    const keydown = (event: KeyboardEvent) => {
      const state = useShortcutStore.getState();
      if (!state.ready || event.defaultPrevented || event.isComposing || shortcutOverlayOpen()) return;
      const target = event.target instanceof Element ? event.target : null;
      if (target?.closest('input, textarea, select, [contenteditable="true"], [role="textbox"]')) return;
      for (const [id, run] of Object.entries(latest.current)) {
        if (!matchesShortcut(event, state.bindings[id])) continue;
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
