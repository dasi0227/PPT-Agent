import { RunPayload, RunScope } from '../../api/types';

export type InteractionMode = 'outline' | 'page' | 'overview' | 'repo';
export type SubMode = 'normal' | 'talk' | 'ask';

export interface OutlineOpts {
  brief?: string;
  slide_count?: number;
  language?: string;
}

export interface MapInput {
  interactionMode: InteractionMode;
  subMode: SubMode;
  hasOutline: boolean;          // slides.length > 0
  currentPageHasHtml: boolean;
  currentPage: number;
  targetPageIndex: number | null;   // Page 模式：null=当前页，number=指定页
  instruction: string;
  outlineOpts?: OutlineOpts;
}

// smartDefault 只影响模式切换器的初始高亮：空项目 → Outline，其余 → Page。
// （见 10-interaction-modes §3：有大纲无论当前页是否有 html 都默认 Page）
export function smartDefault(hasOutline: boolean): InteractionMode {
  return hasOutline ? 'page' : 'outline';
}

function outlineFields(opts?: OutlineOpts): Partial<RunPayload> {
  const fields: Partial<RunPayload> = {};
  if (opts?.brief !== undefined) fields.brief = opts.brief;
  if (opts?.slide_count !== undefined) fields.slide_count = opts.slide_count;
  if (opts?.language !== undefined) fields.language = opts.language;
  return fields;
}

// mapModeToPayload 是 10-interaction-modes §4 映射矩阵的唯一代码化实现。
// 斜杠命令在 Composer 层拦截，不进入本函数（语义交后端解析）。
export function mapModeToPayload(input: MapInput): RunPayload {
  const {
    interactionMode,
    subMode,
    hasOutline,
    currentPageHasHtml,
    currentPage,
    targetPageIndex,
    instruction,
    outlineOpts,
  } = input;

  const base = { instruction };

  // Talk 副模式：只说不做，对所有交互模式统一走 command/talk/current。
  if (subMode === 'talk') {
    return { ...base, kind: 'command', command: 'talk', mode: 'talk', scope: 'current' };
  }

  switch (interactionMode) {
    case 'outline': {
      if (!hasOutline) {
        // 无大纲：首次生成（normal / ask 均走 outline，ask 先澄清再生成）
        return {
          ...base,
          kind: 'outline',
          scope: 'current',
          mode: subMode === 'ask' ? 'ask' : 'normal',
          ...outlineFields(outlineOpts),
        };
      }
      // 有大纲：方案 A —— 退化为 Overview 编辑驱动结构调整
      if (subMode === 'ask') {
        return { ...base, kind: 'command', command: 'ask', mode: 'ask', scope: 'overview' };
      }
      return { ...base, kind: 'edit', scope: 'overview', mode: 'normal' };
    }

    case 'page': {
      const pageIndex = targetPageIndex ?? currentPage;
      const scope: RunScope = targetPageIndex !== null ? 'page' : 'current';
      if (subMode === 'ask') {
        return { ...base, kind: 'command', command: 'ask', mode: 'ask', scope, page_index: pageIndex };
      }
      if (targetPageIndex !== null) {
        // 指定页编辑（该页 html 状态未随输入传入，默认 edit）
        return { ...base, kind: 'edit', scope: 'page', mode: 'normal', page_index: pageIndex };
      }
      // 当前页：有 html → 编辑；无 html → 生成
      if (currentPageHasHtml) {
        return { ...base, kind: 'edit', scope: 'current', mode: 'normal', page_index: pageIndex };
      }
      return { ...base, kind: 'generate', scope: 'current', mode: 'normal', page_index: pageIndex };
    }

    case 'overview': {
      if (subMode === 'ask') {
        return { ...base, kind: 'command', command: 'ask', mode: 'ask', scope: 'overview' };
      }
      return { ...base, kind: 'edit', scope: 'overview', mode: 'normal' };
    }

    case 'repo': {
      if (subMode === 'ask') {
        return { ...base, kind: 'command', command: 'ask', mode: 'ask', scope: 'repo' };
      }
      return { ...base, kind: 'edit', scope: 'repo', mode: 'normal' };
    }
  }
}
