import React, { useEffect, useRef, useState } from 'react';
import { Send } from 'lucide-react';
import { useRunStore } from '../../stores/runStore';
import { useProjectStore } from '../../stores/projectStore';
import { useThreadStore } from '../../stores/threadStore';
import { useDeckStore } from '../../stores/deckStore';
import { useComposerStore } from '../../stores/composerStore';
import { useActiveSession } from './useActiveSession';
import { RunPayload } from '../../api/types';
import { ModeSwitcher } from './ModeSwitcher';
import { mapModeToPayload, smartDefault, InteractionMode } from './modeMapping';

const PLACEHOLDERS: Record<InteractionMode, { empty: string; filled: string }> = {
  outline: {
    empty: '描述你要做的 PPT 主题，例如：给投资人讲我们的 AI 产品，8 页',
    filled: '调整大纲结构，例如：把第 3、4 页合并；在结尾加一页总结',
  },
  page: {
    empty: '编辑当前页，例如：把标题改大一号、配一张示意图',
    filled: '编辑当前页，例如：把标题改大一号、配一张示意图',
  },
  overview: {
    empty: '全局调整，例如：主色改成品牌蓝；所有页统一留白',
    filled: '全局调整，例如：主色改成品牌蓝；所有页统一留白',
  },
  repo: {
    empty: '管理资产，例如：把霓虹卡片组件圆角调大；新建一个深色主题',
    filled: '管理资产，例如：把霓虹卡片组件圆角调大；新建一个深色主题',
  },
};

export const CommandComposer: React.FC = () => {
  const [text, setText] = useState('');
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const { activeProjectId, slidesByProjectId } = useProjectStore();
  const { currentPage } = useDeckStore();
  const createRun = useRunStore((s) => s.createRun);
  const { status } = useActiveSession();
  const ensureActiveThread = useThreadStore((s) => s.ensureActiveThread);
  const {
    interactionMode, subMode, focusNonce,
    setInteractionMode, setSubMode, applySmartDefault,
  } = useComposerStore();

  const slides = activeProjectId ? slidesByProjectId[activeProjectId] || [] : [];
  const hasOutline = slides.length > 0;
  const currentPageHasHtml = !!slides[currentPage]?.html_path;

  const disabled = !activeProjectId || status === 'running' || status === 'needs_input';

  // 智能默认：项目状态变化时更新初始高亮（用户手动切过后不再自动跳）
  useEffect(() => {
    applySmartDefault(smartDefault(hasOutline));
  }, [hasOutline, applySmartDefault]);

  // EmptyState 引导：请求聚焦时把光标落到输入框
  useEffect(() => {
    if (focusNonce > 0) textareaRef.current?.focus();
  }, [focusNonce]);

  const handleSubmit = async () => {
    if (!text.trim() || disabled || !activeProjectId) return;

    const raw = text.trim();

    let payload: RunPayload;
    if (raw.startsWith('/')) {
      // 行首斜杠：语义交后端 command.Parse 权威解析，不本地改写 scope/mode
      payload = { kind: 'edit', instruction: raw };
    } else {
      payload = mapModeToPayload({
        interactionMode,
        subMode,
        hasOutline,
        currentPageHasHtml,
        currentPage,
        targetPageIndex: null,
        instruction: raw,
      });
    }

    let threadId: string;
    try {
      threadId = await ensureActiveThread(activeProjectId);
    } catch (err) {
      console.error('Failed to ensure active thread', err);
      return;
    }

    await createRun(threadId, payload);
    setText('');
  };

  const placeholderSet = PLACEHOLDERS[interactionMode];
  const placeholder =
    status === 'needs_input'
      ? '等待你在上方回复…'
      : hasOutline
        ? placeholderSet.filled
        : placeholderSet.empty;

  return (
    <div className="p-4 border-t border-border bg-background">
      <ModeSwitcher
        interactionMode={interactionMode}
        subMode={subMode}
        onModeChange={(m) => setInteractionMode(m)}
        onSubModeChange={setSubMode}
        disabled={disabled}
      />
      <div className="relative bg-surface rounded-lg border border-border shadow-sm focus-within:border-mode-normal focus-within:ring-1 focus-within:ring-mode-normal transition-all">
        <textarea
          ref={textareaRef}
          value={text}
          onChange={(e) => setText(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
              e.preventDefault();
              handleSubmit();
            }
          }}
          placeholder={placeholder}
          disabled={disabled}
          className="w-full bg-transparent resize-none p-3 max-h-32 min-h-[60px] text-sm text-text-900 placeholder:text-text-400 focus:outline-none disabled:opacity-50"
          rows={2}
        />
        <div className="flex items-center justify-between px-3 pb-2">
          <div className="flex gap-2">
            <span className="text-[10px] text-text-400 bg-background px-1.5 py-0.5 rounded uppercase">{interactionMode}</span>
            {interactionMode === 'page' && (
              <span className="text-[10px] text-text-400 bg-background px-1.5 py-0.5 rounded uppercase">Page {currentPage + 1}</span>
            )}
          </div>
          <button
            onClick={handleSubmit}
            disabled={!text.trim() || disabled}
            aria-label="发送"
            className="p-1.5 bg-mode-normal text-white rounded-md disabled:opacity-50 disabled:bg-text-400 hover:opacity-90 transition-opacity"
          >
            <Send className="w-4 h-4" />
          </button>
        </div>
      </div>
      <div className="mt-2 text-center text-xs text-text-400">
        行首输入 <kbd className="mx-1 px-1 rounded bg-black/5 font-sans border border-border">/current</kbd>
        <kbd className="mx-1 px-1 rounded bg-black/5 font-sans border border-border">/page</kbd>
        <kbd className="mx-1 px-1 rounded bg-black/5 font-sans border border-border">/overview</kbd> 等命令交由后端解析
      </div>
    </div>
  );
};
