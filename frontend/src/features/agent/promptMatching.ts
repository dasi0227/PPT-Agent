import type { ComponentReference, MaterializationState, Prompt, PromptTag } from '../../api/types';

export const MAX_COMPONENT_MENTIONS = 8;
export const MAX_PAGE_MENTIONS = 8;

export const promptTagLabels: Record<PromptTag, string> = {
  identity: '身份',
  deliverable: '交付',
  constraint: '约束',
  git: 'Git',
  review: '审查',
  other: '其它',
};

export const promptTagOrder = Object.keys(promptTagLabels) as PromptTag[];

export interface PromptTrigger {
  start: number;
  end: number;
  query: string;
}

export type ComponentTrigger = PromptTrigger;
export type PageTrigger = PromptTrigger;
export type SummaryTrigger = PromptTrigger;
export type CommandTrigger = PromptTrigger;

export type SlashCommandId =
  | 'plan'
  | 'ask'
  | 'talk'
  | 'kickoff'
  | 'handoff'
  | 'commit'
  | 'polish'
  | 'model'
  | 'target';

export type SlashCommandGroup = '模式' | '操作' | '设置';
export type CommandMenuLevel = 'root' | 'model' | 'target';

export interface SlashCommand {
  id: SlashCommandId;
  name: string;
  ariaLabel: string;
  description: string;
  group: SlashCommandGroup;
  submenu?: 'model' | 'target';
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
}

export interface CommandMenuKeyResult {
  level: CommandMenuLevel;
  activeIndex: number;
  action: 'none' | 'select' | 'close';
}

export const slashCommands: SlashCommand[] = [
  { id: 'plan', name: 'plan', ariaLabel: '计划模式', description: '切换到计划模式', group: '模式' },
  { id: 'ask', name: 'ask', ariaLabel: '审问模式', description: '切换到审问模式', group: '模式' },
  { id: 'talk', name: 'talk', ariaLabel: '聊天模式', description: '切换到聊天模式', group: '模式' },
  { id: 'kickoff', name: 'kickoff', ariaLabel: '启动简报', description: '生成交给新 Agent 的启动 prompt', group: '操作' },
  { id: 'handoff', name: 'handoff', ariaLabel: '交接简报', description: '生成上下文交接 prompt', group: '操作' },
  { id: 'commit', name: 'commit', ariaLabel: '提交', description: '执行一次 Git 提交', group: '操作' },
  { id: 'polish', name: 'polish', ariaLabel: '润色', description: '润色当前输入内容', group: '操作' },
  { id: 'model', name: 'model', ariaLabel: '切换模型', description: '选择对话使用的模型', group: '设置', submenu: 'model' },
  { id: 'target', name: 'target', ariaLabel: '切换目标', description: '选择生成目标范围与对象', group: '设置', submenu: 'target' },
];

export interface PageMentionCandidate {
  slideId: string;
  ordinal: number;
  title: string;
  keyMessage: string;
  specState: 'pending' | 'ready';
  htmlState: MaterializationState;
}

export function findPromptTrigger(text: string, caret: number): PromptTrigger | null {
  if (caret < 0 || caret > text.length) return null;
  const before = text.slice(0, caret);
  const match = before.match(/(?:^|[ \n])([%％])([^ \n%％]*)$/);
  if (!match) return null;
  return {
    start: caret - match[1].length - match[2].length,
    end: caret,
    query: match[2],
  };
}

export function findComponentTrigger(text: string, caret: number): ComponentTrigger | null {
  if (caret < 0 || caret > text.length) return null;
  const before = text.slice(0, caret);
  const match = before.match(/(?:^|[ \n])([¥$])([^ \n¥$]*)$/);
  if (!match) return null;
  return {
    start: caret - match[1].length - match[2].length,
    end: caret,
    query: match[2],
  };
}

export function findPageTrigger(text: string, caret: number): PageTrigger | null {
  if (caret < 0 || caret > text.length) return null;
  const before = text.slice(0, caret);
  const match = before.match(/(?:^|[ \n])(#)([^ \n#]*)$/);
  if (!match) return null;
  return {
    start: caret - match[1].length - match[2].length,
    end: caret,
    query: match[2],
  };
}

export function findSummaryTrigger(text: string, caret: number): SummaryTrigger | null {
  if (caret < 0 || caret > text.length) return null;
  const before = text.slice(0, caret);
  const match = before.match(/(?:^|[ \n])(@)([^ \n@]*)$/);
  if (!match) return null;
  return {
    start: caret - match[1].length - match[2].length,
    end: caret,
    query: match[2],
  };
}

export function findCommandTrigger(text: string, caret: number): CommandTrigger | null {
  if (caret < 0 || caret > text.length) return null;
  const before = text.slice(0, caret);
  const match = before.match(/(?:^|[ \n])(\/)([^ \n/]*)$/);
  if (!match) return null;
  return {
    start: caret - match[1].length - match[2].length,
    end: caret,
    query: match[2],
  };
}

export function resolveSlashCommands(availability: SlashCommandAvailability): ResolvedSlashCommand[] {
  return slashCommands.map((command) => {
    if (!['kickoff', 'handoff', 'commit', 'polish'].includes(command.id)) {
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
  if (!normalized) return commands;
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
  candidateCount: number,
): CommandMenuKeyResult {
  if (key === 'Escape') {
    return level === 'root'
      ? { level, activeIndex, action: 'close' }
      : { level: 'root', activeIndex: 0, action: 'none' };
  }
  if (key === 'Enter') {
    return { level, activeIndex, action: candidateCount > 0 ? 'select' : 'none' };
  }
  if (candidateCount <= 0) {
    return { level, activeIndex: 0, action: 'none' };
  }
  const direction = key === 'ArrowDown' ? 1 : -1;
  return {
    level,
    activeIndex: (activeIndex + direction + candidateCount) % candidateCount,
    action: 'none',
  };
}

function includes(value: string, query: string): boolean {
  return value.toLocaleLowerCase().includes(query.toLocaleLowerCase());
}

export function matchPrompts(prompts: Prompt[], query: string): Prompt[] {
  const enabledPrompts = prompts.filter((prompt) => !prompt.disabled);
  return enabledPrompts
    .map((prompt) => {
      const rank = !query
        ? 0
        : includes(prompt.name, query)
        ? 0
        : prompt.tags.some((tag) => includes(tag, query) || includes(promptTagLabels[tag], query))
          ? 1
          : includes(prompt.desc, query)
            ? 2
            : includes(prompt.value, query)
              ? 3
              : 4;
      return { prompt, rank };
    })
    .filter(({ rank }) => rank < 4)
    .sort((left, right) => (
      left.rank - right.rank
      || right.prompt.updated_at - left.prompt.updated_at
      || left.prompt.name.localeCompare(right.prompt.name, 'zh-CN')
    ))
    .slice(0, 8)
    .map(({ prompt }) => prompt);
}

export function matchComponents(components: ComponentReference[], query: string): ComponentReference[] {
  const enabled = components.filter((component) => !component.disabled);
  return enabled
    .map((component) => {
      const rank = !query
        ? 0
        : includes(component.name, query) || includes(component.id, query)
          ? 0
          : component.tags.some((tag) => includes(tag, query))
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
