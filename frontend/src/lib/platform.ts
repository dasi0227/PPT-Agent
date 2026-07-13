export function isMac(): boolean {
  if (typeof navigator === 'undefined') return false;
  return /Mac|iPhone|iPad|iPod/.test(navigator.platform);
}

export function submitShortcutLabel(): string {
  return isMac() ? '⌘ + Enter' : 'Ctrl + Enter';
}
