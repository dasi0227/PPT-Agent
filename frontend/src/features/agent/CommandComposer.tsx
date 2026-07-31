import React, { useEffect, useRef, useState } from 'react';
import { Send, WandSparkles } from 'lucide-react';
import { useRunStore } from '../../stores/runStore';
import { useProjectStore } from '../../stores/projectStore';
import { useThreadStore } from '../../stores/threadStore';
import { useDeckStore } from '../../stores/deckStore';
import { useComposerStore } from '../../stores/composerStore';
import { useActiveSession } from './useActiveSession';
import { RunPayload } from '../../api/types';
import { ModeSwitcher } from './ModeSwitcher';
import { InteractionMode, mapModeToPayload } from './modeMapping';
import { isMac } from '../../lib/platform';
import { cn } from '../../lib/utils';

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

  const submit = async (generateDeck = false) => {
    if (disabled || !activeProjectId || (!generateDeck && !text.trim())) return;

    let projectId = activeProjectId;
    const slides = projectId && projectId !== 'new-pending' ? slidesByProjectId[projectId] || [] : [];
    const hasSlides = slides.length > 0;
    const raw = text.trim();
    const currentPageHasHtml = !!slides[currentPage]?.html_path;
    let payload: RunPayload;

    if (generateDeck) {
      if (!hasSlides) return;
      payload = {
        kind: 'generate',
        scope: 'overview',
        instruction: raw || '基于当前大纲生成整套 HTML PPT',
      };
    } else if (raw.startsWith('/')) {
      // 斜杠命令的 scope/mode/command/page_index 只由后端 command.Parse 决定。
      payload = { kind: 'edit', instruction: raw };
    } else {
      payload = mapModeToPayload({
        interactionMode,
        subMode,
        hasOutline: hasSlides,
        currentPageHasHtml,
        currentPage,
        targetPageIndex: null,
        instruction: raw,
      });
    }

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

  const handleSubmit = () => void submit(false);
  const handleGenerateDeck = () => void submit(true);

  const slides = activeProjectId && activeProjectId !== 'new-pending' ? slidesByProjectId[activeProjectId] || [] : [];
  const hasSlides = slides.length > 0;
  const derivedMode = interactionMode;
  const placeholder = PLACEHOLDERS[derivedMode]?.[hasSlides ? 'filled' : 'empty'] || '';

  const ringColorMap: Record<InteractionMode, string> = {
    outline: 'ring-mode-outline border-mode-outline',
    page: 'ring-mode-page border-mode-page',
    overview: 'ring-mode-overview border-mode-overview',
    repo: 'ring-mode-repo border-mode-repo',
  };
  const activeRing = ringColorMap[derivedMode] || 'ring-border border-border';

  const btnColorMap: Record<InteractionMode, string> = {
    outline: 'bg-mode-outline',
    page: 'bg-mode-page',
    overview: 'bg-mode-overview',
    repo: 'bg-mode-repo',
  };
  const activeBtn = btnColorMap[derivedMode] || 'bg-mode-outline';

  return (
    <div className="p-4 border-t border-border bg-background">
      <div className={cn("relative bg-surface rounded-lg border shadow-sm ring-1 transition-all", activeRing)}>
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
          <div className="flex min-w-0 items-center gap-2">
            <ModeSwitcher
              interactionMode={interactionMode}
              subMode={subMode}
              onModeChange={(m) => useComposerStore.getState().setInteractionMode(m)}
              onSubModeChange={(m) => useComposerStore.getState().setSubMode(m)}
            />
            <button
              type="button"
              onClick={handleGenerateDeck}
              disabled={!hasSlides || disabled}
              className="inline-flex shrink-0 items-center gap-1 rounded border border-mode-overview/40 px-2 py-1 text-xs font-medium text-mode-overview transition-colors hover:bg-mode-overview/10 disabled:cursor-not-allowed disabled:opacity-40"
              title={hasSlides ? '基于当前大纲生成全部页面 HTML' : '请先生成大纲'}
            >
              <WandSparkles className="h-3.5 w-3.5" />
              整套生成
            </button>
          </div>
          <button
            onClick={handleSubmit}
            disabled={!text.trim() || disabled}
            className={cn("p-1.5 text-white rounded-md disabled:opacity-50 disabled:bg-text-400 hover:opacity-90 transition-opacity", activeBtn)}
            aria-label="发送"
          >
            <Send className="w-4 h-4" />
          </button>
        </div>
      </div>
    </div>
  );
};
