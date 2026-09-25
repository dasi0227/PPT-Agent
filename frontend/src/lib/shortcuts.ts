import catalogJSON from '../../../backend/internal/shortcuts/catalog.json';
import { isMac } from './platform';

export interface ShortcutBinding { trigger?: string; code?: string; primary?: boolean; alt?: boolean; shift?: boolean }
export interface ShortcutDefinition { id: string; group: string; label: string; kind: string; default: ShortcutBinding }
export type ShortcutBindings = Record<string, ShortcutBinding>;
export interface ShortcutSettings { revision: number; bindings: ShortcutBindings }
export const shortcutCatalog: ShortcutDefinition[] = catalogJSON;
export const defaultBindings: ShortcutBindings = Object.fromEntries(shortcutCatalog.map(def => [def.id, def.default]));
export const triggerCharacters = '/@$#%!?&*~^:;=+-_.|\\';
const codePattern = /^(Key[A-Z]|Digit[0-9]|Arrow(Left|Right|Up|Down)|Enter|Space|Equal|Minus|BracketLeft|BracketRight|Backslash|Semicolon|Quote|Comma|Period|Slash|Backquote)$/;
const reservedPrimary = new Set(['KeyA','KeyC','KeyV','KeyX','KeyZ','KeyY','KeyF','KeyL','KeyQ','KeyW','KeyR','KeyN']);
export function bindingSignature(b: ShortcutBinding): string {
  return b.trigger ? `trigger:${b.trigger}` : `${b.code}:${!!b.primary}:${!!b.alt}:${!!b.shift}`;
}
export function validateBindings(bindings: ShortcutBindings): string | null {
  if (Object.keys(bindings).length !== shortcutCatalog.length) return '快捷键配置必须包含所有功能';
  const seen = new Map<string, string>();
  for (const def of shortcutCatalog) {
    const b = bindings[def.id];
    if (!b) return `缺少配置：${def.label}`;
    if (def.kind === 'trigger') {
      if (!b.trigger || b.trigger.length !== 1 || !triggerCharacters.includes(b.trigger) || b.code || b.primary || b.alt || b.shift) return `${def.label}：请输入单个支持的标点符号`;
    } else {
      if (b.trigger || !codePattern.test(b.code ?? '') || (!b.primary && !b.alt) || (b.code === 'Equal' && b.shift)) return `${def.label}：请至少使用 Command/Ctrl 或 Option/Alt，可组合 Shift`;
      if (b.primary && reservedPrimary.has(b.code!)) return `${def.label}：该组合保留给系统编辑或浏览器操作`;
    }
    const signature = bindingSignature(b);
    if (seen.has(signature)) return `${seen.get(signature)} 与 ${def.label} 的绑定冲突`;
    seen.set(signature, def.label);
  }
  return null;
}
export interface ShortcutEvent {
  code?: string; key?: string; metaKey: boolean; ctrlKey: boolean; altKey: boolean; shiftKey: boolean; isComposing?: boolean;
}
export function eventCode(event: ShortcutEvent): string {
  if (event.code) return event.code;
  const key = event.key ?? '';
  if (/^[a-z]$/i.test(key)) return `Key${key.toUpperCase()}`;
  if (/^[0-9]$/.test(key)) return `Digit${key}`;
  return ({ '+': 'Equal', '=': 'Equal', '-': 'Minus', ' ': 'Space' } as Record<string,string>)[key] ?? key;
}
export function recordShortcut(event: ShortcutEvent, mac = isMac()): ShortcutBinding {
  return { code: eventCode(event), primary: mac ? event.metaKey : event.ctrlKey, alt: event.altKey, shift: eventCode(event) !== 'Equal' && event.shiftKey };
}
export function matchesShortcut(event: ShortcutEvent, binding: ShortcutBinding | undefined, mac = isMac()): boolean {
  if (!binding?.code || event.isComposing || (mac ? event.ctrlKey : event.metaKey)) return false;
  return bindingSignature(recordShortcut(event, mac)) === bindingSignature(binding);
}
export function shortcutLabel(b: ShortcutBinding | undefined, mac = isMac()): string {
  if (!b) return '';
  if (b.trigger) return b.trigger;
  const names: Record<string,string> = { ArrowLeft:'←',ArrowRight:'→',ArrowUp:'↑',ArrowDown:'↓',Equal:'+',Minus:'−',Space:'Space',BracketLeft:'[',BracketRight:']',Backslash:'\\',Semicolon:';',Quote:"'",Comma:',',Period:'.',Slash:'/',Backquote:'`' };
  const key = names[b.code ?? ''] ?? b.code?.replace(/^(Key|Digit)/, '') ?? '';
  return [b.primary && (mac ? '⌘' : 'Ctrl'), b.alt && (mac ? '⌥' : 'Alt'), b.shift && 'Shift', key].filter(Boolean).join(' + ');
}
export function shortcutOverlayOpen(): boolean {
  return !!document.querySelector('[role="dialog"][data-state="open"], [role="alertdialog"][data-state="open"], [role="menu"][data-state="open"], [role="listbox"], [data-composer-menu]');
}
