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
export type SlashTrigger = PromptTrigger;

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
  const match = before.match(/(?:^|[ \n])([$¥])([^ \n$¥]*)$/);
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
  const match = before.match(/(?:^|[ \n])(#)([^ \n#]*)$/);
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
  const match = before.match(/(?:^|[ \n])(@)([^ \n@]*)$/);
  if (!match) return null;
  return {
    start: caret - match[1].length - match[2].length,
    end: caret,
    query: match[2],
  };
}

export function findSlashTrigger(text: string, caret: number): SlashTrigger | null {
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
