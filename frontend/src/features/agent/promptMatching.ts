import type { ComponentReference, HTMLState, Snippet } from '../../api/types';
import type { WorkspaceActionId } from '../../lib/useWorkspaceAction';

export const MAX_COMPONENT_MENTIONS = 8;
export const MAX_PAGE_MENTIONS = 8;

export interface InputTrigger {
  start: number;
  end: number;
  query: string;
}

export type ComponentTrigger = InputTrigger;
export type PageTrigger = InputTrigger;
export type SummaryTrigger = InputTrigger;
export type CommandTrigger = InputTrigger;

export type SlashCommandId =
  | WorkspaceActionId
  | 'execute'
  | 'chat'
  | 'grill'
  | 'plan'
  | 'handoff'
  | 'compact'
  | 'commit'
  | 'polish'
	| 'rename'
  | 'mode'
  | 'model'
  | 'file'
  | 'skill'
  | 'theme'
  | 'appearance'
  | 'target';

export type SlashCommandGroup = '命令' | '配置' | '操作';
export type CommandMenuLevel = 'root' | 'mode' | 'model' | 'target' | 'skill' | 'theme';

export interface SlashCommand {
  id: SlashCommandId;
  name: string;
  ariaLabel: string;
  description: string;
  group: SlashCommandGroup;
  submenu?: Exclude<CommandMenuLevel, 'root'>;
  hiddenByDefault?: boolean;
}

export interface ResolvedSlashCommand extends SlashCommand {
  disabled: boolean;
  disabledReason?: string;
}

export interface SlashCommandAvailability {
  runActive: boolean;
  emptyProject: boolean;
  operationBusy: boolean;
  hasPolishText: boolean;
  compactAvailable?: boolean;
  compactUnavailableReason?: string;
}

export interface CommandMenuKeyResult {
  level: CommandMenuLevel;
  activeIndex: number;
  action: 'none' | 'select' | 'close';
}

export const slashCommands: SlashCommand[] = [
  { id: 'handoff', name: 'handoff', ariaLabel: '交接简报', description: '生成交给新会话的交接 Prompt', group: '命令' },
  { id: 'compact', name: 'compact', ariaLabel: '压缩上下文', description: '压缩当前会话的历史记录', group: '命令' },
  { id: 'commit', name: 'commit', ariaLabel: '提交', description: '执行一次 Git 提交', group: '命令' },
  { id: 'polish', name: 'polish', ariaLabel: '润色', description: '润色当前输入内容', group: '命令' },
	{ id: 'rename', name: 'rename', ariaLabel: '会话命名', description: '选择自动或手动命名', group: '命令' },
  { id: 'mode', name: 'mode', ariaLabel: '切换模式', description: '选择对话模式', group: '配置', submenu: 'mode' },
  { id: 'execute', name: 'create', ariaLabel: '开发模式', description: '切换到开发模式', group: '配置', hiddenByDefault: true },
  { id: 'chat', name: 'chat', ariaLabel: '讨论模式', description: '切换到讨论模式', group: '配置', hiddenByDefault: true },
  { id: 'grill', name: 'grill', ariaLabel: '盘问模式', description: '切换到盘问模式', group: '配置', hiddenByDefault: true },
  { id: 'plan', name: 'plan', ariaLabel: '计划模式', description: '切换到计划模式', group: '配置', hiddenByDefault: true },
  { id: 'model', name: 'model', ariaLabel: '切换模型', description: '选择对话使用的模型', group: '配置', submenu: 'model' },
  { id: 'target', name: 'target', ariaLabel: '切换目标', description: '选择生成目标范围与对象', group: '配置', submenu: 'target' },
  { id: 'file', name: 'file', ariaLabel: '选择文件', description: '打开文件选择弹窗', group: '配置' },
  { id: 'skill', name: 'skill', ariaLabel: '选择技能', description: '选择本次使用的技能', group: '配置', submenu: 'skill' },
  { id: 'appearance', name: 'appearance', ariaLabel: '切换外观', description: '切换浅色或深色模式', group: '操作' },
  { id: 'theme', name: 'theme', ariaLabel: '选择主题', description: '切换演示文稿主题', group: '操作', submenu: 'theme' },
  { id: 'play', name: 'play', ariaLabel: '放映', description: '全屏放映演示文稿', group: '操作' },
  { id: 'export', name: 'export', ariaLabel: '导出', description: '打开导出弹窗', group: '操作' },
  { id: 'setting', name: 'setting', ariaLabel: '设置', description: '打开设置页面', group: '操作' },
  { id: 'repo', name: 'repo', ariaLabel: '仓库', description: '打开仓库页面', group: '操作' },
  { id: 'home', name: 'home', ariaLabel: '主页', description: '打开欢迎页', group: '操作' },
];

export interface PageMentionCandidate {
  slideId: string;
  ordinal: number;
  title: string;
  keyMessage: string;
  specState: 'pending' | 'ready';
  htmlState: HTMLState;
}

function findInputTrigger(text: string, caret: number, symbol: string): InputTrigger | null {
  if (caret < 0 || caret > text.length || !symbol) return null;
  const before = text.slice(0, caret);
  const start = Math.max(before.lastIndexOf(' '), before.lastIndexOf('\n')) + 1;
  const token = before.slice(start);
  // Input-method variants belong to the same configurable trigger, not separate bindings.
  const aliases = symbol === '$' ? ['$', '¥'] : symbol === '%' ? ['%', '％'] : [symbol];
  if (!aliases.includes(token[0]) || aliases.some(value => token.slice(1).includes(value))) return null;
  return { start, end: caret, query: token.slice(1) };
}
export function findSnippetTrigger(text: string, caret: number, symbol = '%'): InputTrigger | null {
  return findInputTrigger(text, caret, symbol);
}
export function findComponentTrigger(text: string, caret: number, symbol = '$'): ComponentTrigger | null {
  return findInputTrigger(text, caret, symbol);
}
export function findPageTrigger(text: string, caret: number, symbol = '#'): PageTrigger | null {
  return findInputTrigger(text, caret, symbol);
}
export function findSummaryTrigger(text: string, caret: number, symbol = '@'): SummaryTrigger | null {
  return findInputTrigger(text, caret, symbol);
}
export function findCommandTrigger(text: string, caret: number, symbol = '/'): CommandTrigger | null {
  return findInputTrigger(text, caret, symbol);
}

export function resolveSlashCommands(availability: SlashCommandAvailability): ResolvedSlashCommand[] {
  return slashCommands.map((command) => {
    if (command.id === 'compact') {
      const disabledReason = availability.runActive
        ? '任务运行中'
        : availability.operationBusy
          ? '其他操作进行中'
          : availability.compactAvailable ? undefined : availability.compactUnavailableReason ?? '当前无法压缩上下文';
      return { ...command, disabled: Boolean(disabledReason), disabledReason };
    }
    if (!['handoff', 'commit', 'polish'].includes(command.id)) {
      return { ...command, disabled: false };
    }
    const disabledReason = availability.runActive
      ? '任务运行中'
      : availability.emptyProject
        ? '当前为空项目'
        : availability.operationBusy
          ? '其他操作进行中'
          : command.id === 'polish' && !availability.hasPolishText
            ? '请输入需要润色的内容'
            : undefined;
    return { ...command, disabled: Boolean(disabledReason), disabledReason };
  });
}

export function matchSlashCommands(commands: ResolvedSlashCommand[], query: string): ResolvedSlashCommand[] {
  const normalized = query.trim().toLocaleLowerCase();
  if (!normalized) return commands.filter((command) => !command.hiddenByDefault);
  return commands.filter((command) => {
    const name = command.name.toLocaleLowerCase();
    if (name.startsWith(normalized)) return true;
    let cursor = 0;
    for (const character of normalized) {
      cursor = name.indexOf(character, cursor);
      if (cursor < 0) return false;
      cursor += 1;
    }
    return true;
  });
}

export function navigateCommandMenu(
  level: CommandMenuLevel,
  activeIndex: number,
  key: 'ArrowUp' | 'ArrowDown' | 'Enter' | 'Escape',
  candidateDisabled: readonly boolean[],
): CommandMenuKeyResult {
  const candidateCount = candidateDisabled.length;
  if (key === 'Escape') {
    return level === 'root'
      ? { level, activeIndex, action: 'close' }
      : { level: 'root', activeIndex: 0, action: 'none' };
  }
  if (key === 'Enter') {
    const canSelect = activeIndex >= 0
      && activeIndex < candidateCount
      && !candidateDisabled[activeIndex];
    return { level, activeIndex, action: canSelect ? 'select' : 'none' };
  }
  if (candidateCount <= 0) {
    return { level, activeIndex: 0, action: 'none' };
  }
  const direction = key === 'ArrowDown' ? 1 : -1;
  let nextIndex = activeIndex >= 0 && activeIndex < candidateCount
    ? activeIndex
    : direction > 0 ? -1 : 0;
  for (let step = 0; step < candidateCount; step += 1) {
    nextIndex = (nextIndex + direction + candidateCount) % candidateCount;
    if (!candidateDisabled[nextIndex]) {
      return { level, activeIndex: nextIndex, action: 'none' };
    }
  }
  return { level, activeIndex: -1, action: 'none' };
}

function includes(value: string, query: string): boolean {
  return value.toLocaleLowerCase().includes(query.toLocaleLowerCase());
}

export function matchSnippets(snippets: Snippet[], query: string, tagLabels: Readonly<Record<string, string>> = {}): Snippet[] {
  const enabledSnippets = snippets.filter((snippet) => !snippet.disabled && snippet.content_state === 'ready');
  return enabledSnippets
    .map((snippet) => {
      const rank = !query
        ? 0
        : includes(snippet.name, query)
        ? 0
        : snippet.tags.some((tag) => includes(tag, query) || includes(tagLabels[tag] ?? '', query))
          ? 1
          : includes(snippet.description, query)
            ? 2
            : includes(snippet.content, query)
              ? 3
              : 4;
      return { snippet, rank };
    })
    .filter(({ rank }) => rank < 4)
    .sort((left, right) => (
      left.rank - right.rank
      || right.snippet.updated_at - left.snippet.updated_at
      || left.snippet.name.localeCompare(right.snippet.name, 'zh-CN')
    ))
    .slice(0, 8)
    .map(({ snippet }) => snippet);
}

export function matchComponents(components: ComponentReference[], query: string, tagLabels: Readonly<Record<string, string>> = {}): ComponentReference[] {
  const enabled = components.filter((component) => !component.disabled && component.content_state === 'ready');
  return enabled
    .map((component) => {
      const rank = !query
        ? 0
        : includes(component.name, query) || includes(component.id, query)
          ? 0
          : component.tags.some((tag) => includes(tag, query) || includes(tagLabels[tag] ?? '', query))
            ? 1
            : includes(component.description, query)
              ? 2
              : 3;
      return { component, rank };
    })
    .filter(({ rank }) => rank < 3)
    .sort((left, right) => (
      left.rank - right.rank
      || left.component.name.localeCompare(right.component.name, 'zh-CN')
      || left.component.id.localeCompare(right.component.id)
    ))
    .slice(0, MAX_COMPONENT_MENTIONS)
    .map(({ component }) => component);
}

export function pageDisplayName(page: Pick<PageMentionCandidate, 'ordinal' | 'title'>): string {
  const title = page.title.trim();
  return title ? `Page ${page.ordinal} · ${title}` : `Page ${page.ordinal}`;
}

export function matchPages(pages: PageMentionCandidate[], query: string): PageMentionCandidate[] {
  const normalized = query.trim().toLocaleLowerCase();
  if (!normalized) return [...pages].sort((a, b) => a.ordinal - b.ordinal).slice(0, MAX_PAGE_MENTIONS);
  return pages
    .map((page) => {
      const titleMatch = includes(page.title, normalized);
      const ordinal = String(page.ordinal);
      const pageLabel = `page ${ordinal}`;
      const ordinalMatch = ordinal === normalized || pageLabel.includes(normalized);
      return { page, rank: titleMatch ? 0 : ordinalMatch ? 1 : 2 };
    })
    .filter(({ rank }) => rank < 2)
    .sort((a, b) => a.rank - b.rank || a.page.ordinal - b.page.ordinal)
    .slice(0, MAX_PAGE_MENTIONS)
    .map(({ page }) => page);
}
