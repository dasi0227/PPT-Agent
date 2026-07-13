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
import { mapModeToPayload, InteractionMode } from './modeMapping';
import { isMac, submitShortcutLabel } from '../../lib/platform';

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
  const [isComposing, setIsComposing] = useState(false);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const { activeProjectId, slidesByProjectId, createProject, finalizePendingNewProject } = useProjectStore();
  const { currentPage } = useDeckStore();
  const { ensureActiveThread } = useThreadStore();
  const { createRun } = useRunStore();
  const { status: runStatus } = useActiveSession();
  const { interactionMode, subMode, focusNonce } = useComposerStore();

  const disabled = !activeProjectId || runStatus === 'running' || runStatus === 'needs_input';

  useEffect(() => {
    if (focusNonce > 0) textareaRef.current?.focus();
  }, [focusNonce]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.nativeEvent.isComposing || isComposing) return;
    
    if (e.key === 'Enter') {
      if ((isMac() ? e.metaKey : e.ctrlKey)) {
        e.preventDefault();
        handleSubmit();
      }
    }
  };

  const handleSubmit = async () => {
    if (!text.trim() || disabled || !activeProjectId) return;

    let projectId = activeProjectId;
    const slides = projectId && projectId !== 'new-pending' ? slidesByProjectId[projectId] || [] : [];
    const hasSlides = slides.length > 0;
    
    const raw = text.trim();
    const payload: RunPayload = mapModeToPayload({
      interactionMode,
      subMode,
      hasOutline: hasSlides,
      currentPageHasHtml: true, // simplified
      currentPage,
      targetPageIndex: null,
      instruction: raw,
    });

    if (projectId === 'new-pending') {
      try {
        const proj = await createProject(raw, payload.brief, payload.slide_count, payload.language);
        projectId = proj.id;
        finalizePendingNewProject(proj.id);
      } catch (err) {
        console.error('Failed to create project', err);
        return;
      }
    }

    const threadId = await ensureActiveThread(projectId);
    await createRun(threadId, payload);
    setText('');
  };

  const slides = activeProjectId && activeProjectId !== 'new-pending' ? slidesByProjectId[activeProjectId] || [] : [];
  const hasSlides = slides.length > 0;
  const derivedMode = interactionMode;
  const placeholder = PLACEHOLDERS[derivedMode]?.[hasSlides ? 'filled' : 'empty'] || '';

  return (
    <div className="p-4 border-t border-border bg-background">
      <div className="relative bg-surface rounded-lg border border-border shadow-sm focus-within:border-mode-normal focus-within:ring-1 focus-within:ring-mode-normal transition-all">
        <textarea
          ref={textareaRef}
          value={text}
          onChange={(e) => setText(e.target.value)}
          onKeyDown={handleKeyDown}
          onCompositionStart={() => setIsComposing(true)}
          onCompositionEnd={() => setIsComposing(false)}
          placeholder={placeholder}
          disabled={disabled}
          className="w-full bg-transparent resize-none p-3 max-h-32 min-h-[60px] text-sm text-text-900 placeholder:text-text-400 focus:outline-none disabled:opacity-50"
          rows={2}
        />
        <div className="flex items-center justify-between px-3 pb-2">
          <ModeSwitcher 
            interactionMode={interactionMode} 
            subMode={subMode} 
            onModeChange={(m) => useComposerStore.getState().setInteractionMode(m)} 
            onSubModeChange={(m) => useComposerStore.getState().setSubMode(m)} 
          />
          <button
            onClick={handleSubmit}
            disabled={!text.trim() || disabled}
            className="p-1.5 bg-mode-normal text-white rounded-md disabled:opacity-50 disabled:bg-text-400 hover:opacity-90 transition-opacity"
          >
            <Send className="w-4 h-4" />
          </button>
        </div>
      </div>
      <div className="mt-2 flex items-center justify-between">
        <div className="text-center text-xs text-text-400">
          行首输入 <kbd className="mx-1 px-1 rounded bg-black/5 font-sans border border-border">/current</kbd>
          <kbd className="mx-1 px-1 rounded bg-black/5 font-sans border border-border">/page</kbd>
          <kbd className="mx-1 px-1 rounded bg-black/5 font-sans border border-border">/overview</kbd> 等命令交由后端解析
        </div>
        <div className="text-xs text-text-400">
          <kbd className="px-1 rounded bg-black/5 font-sans border border-border">{submitShortcutLabel()}</kbd> 发送
        </div>
      </div>
    </div>
  );
};
