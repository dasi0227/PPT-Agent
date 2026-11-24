import { isMac } from './platform';

export interface ShortcutBinding { trigger?: string; code?: string; primary?: boolean; alt?: boolean; shift?: boolean }
export interface ShortcutDefinition { id: string; field: string; group: string; label: string; kind: string; default: ShortcutBinding }
export type ShortcutBindings = Record<string, ShortcutBinding>;
export interface ShortcutSettings { revision: number; bindings: ShortcutBindings }
export const shortcutCatalog: ShortcutDefinition[] = [
  { id: 'trigger.command', field: 'command_trigger', group: '输入触发符', label: '命令菜单', kind: 'trigger', default: { trigger: '/' } },
  { id: 'trigger.summary', field: 'reference_trigger', group: '输入触发符', label: '综合引用', kind: 'trigger', default: { trigger: '@' } },
  { id: 'trigger.component', field: 'component_trigger', group: '输入触发符', label: '组件引用', kind: 'trigger', default: { trigger: '$' } },
  { id: 'trigger.page', field: 'slide_trigger', group: '输入触发符', label: '页面引用', kind: 'trigger', default: { trigger: '#' } },
  { id: 'trigger.snippet', field: 'snippet_trigger', group: '输入触发符', label: '短语', kind: 'trigger', default: { trigger: '%' } },
  { id: 'deck.previous', field: 'previous_slide', group: '演示文稿', label: '上一页', kind: 'shortcut', default: { code: 'ArrowLeft', primary: true } },
  { id: 'deck.next', field: 'next_slide', group: '演示文稿', label: '下一页', kind: 'shortcut', default: { code: 'ArrowRight', primary: true } },
  { id: 'deck.zoom_in', field: 'zoom_in', group: '演示文稿', label: '放大', kind: 'shortcut', default: { code: 'Equal', primary: true } },
  { id: 'deck.zoom_out', field: 'zoom_out', group: '演示文稿', label: '缩小', kind: 'shortcut', default: { code: 'Minus', primary: true } },
  { id: 'deck.overview', field: 'toggle_overview', group: '演示文稿', label: '总览', kind: 'shortcut', default: { code: 'KeyO', primary: true } },
  { id: 'deck.export', field: 'export_deck', group: '演示文稿', label: '导出', kind: 'shortcut', default: { code: 'KeyP', primary: true } },
  { id: 'deck.view', field: 'toggle_view', group: '演示文稿', label: '切换视图', kind: 'shortcut', default: { code: 'KeyU', primary: true } },
  { id: 'deck.form', field: 'toggle_source', group: '演示文稿', label: '切换 HTML 预览与源码', kind: 'shortcut', default: { code: 'KeyI', primary: true } },
  { id: 'deck.theme', field: 'switch_theme', group: '演示文稿', label: '切换主题', kind: 'shortcut', default: { code: 'KeyT', primary: true } },
  { id: 'composer.submit', field: 'submit_request', group: '输入框', label: '提交请求', kind: 'shortcut', default: { code: 'Enter', primary: true } },
  { id: 'composer.suggestion1', field: 'select_suggestion', group: '输入框', label: '填入第 1 条建议', kind: 'shortcut', default: { code: 'Digit1', alt: true } },
  { id: 'composer.suggestion2', field: 'select_suggestion', group: '输入框', label: '填入第 2 条建议', kind: 'shortcut', default: { code: 'Digit2', alt: true } },
  { id: 'composer.suggestion3', field: 'select_suggestion', group: '输入框', label: '填入第 3 条建议', kind: 'shortcut', default: { code: 'Digit3', alt: true } },
];
export const defaultBindings: ShortcutBindings = Object.fromEntries(shortcutCatalog.map(def => [def.id, def.default]));
export const triggerCharacters = '/~!@#$%^&*>\\+';
const modifierCode = /^(Primary|Option|Alt|AltLeft|AltRight|AltGraph|Shift|ShiftLeft|ShiftRight|Ctrl|Control|ControlLeft|ControlRight|Command|Meta|MetaLeft|MetaRight|OSLeft|OSRight|Fn)$/;
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
      if (b.trigger || !b.code || /[+\s]/.test(b.code) || modifierCode.test(b.code) || !!b.primary === !!b.alt || b.shift) return `${def.label}：请使用一个 Command/Ctrl 或 Option/Alt 加一个普通键`;
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
  return { code: eventCode(event), primary: mac ? event.metaKey : event.ctrlKey, alt: event.altKey, shift: event.shiftKey };
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

export function fromShortcutFields(fields: Record<string, string>): ShortcutBindings {
  return Object.fromEntries(shortcutCatalog.map(def => {
    const value = fields[def.field];
    if (def.kind === 'trigger') return [def.id, { trigger: value }];
    const [modifier, code] = value.split('+');
    return [def.id, { code: def.field === 'select_suggestion' ? def.default.code : code, primary: modifier === 'Primary', alt: modifier !== 'Primary' }];
  }));
}

export function toShortcutFields(bindings: ShortcutBindings): Record<string, string> {
  return Object.fromEntries(shortcutCatalog.map(def => {
    const binding = bindings[def.id];
    const modifier = binding.primary ? 'Primary' : 'Option';
    return [def.field, binding.trigger ?? (def.field === 'select_suggestion' ? modifier : modifier + '+' + binding.code)];
  }));
}
