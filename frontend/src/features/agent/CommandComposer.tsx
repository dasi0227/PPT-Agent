import React, { useEffect, useRef, useState } from 'react';
import { Send, WandSparkles } from 'lucide-react';
import type { CreateRunRequest } from '../../api/types';
import { useComposerStore } from '../../stores/composerStore';
import { useDeckStore } from '../../stores/deckStore';
import { useProjectStore } from '../../stores/projectStore';
import { useRunStore } from '../../stores/runStore';
import { useThreadStore } from '../../stores/threadStore';
import { cn } from '../../lib/utils';
import { isMac } from '../../lib/platform';
import { ModeSwitcher } from './ModeSwitcher';
import { useActiveSession } from './useActiveSession';

function applyShortcut(raw: string, request: CreateRunRequest): CreateRunRequest {
  if (!raw.startsWith('/')) return request;
  const [command, ...rest] = raw.split(/\s+/);
  const instruction = rest.join(' ').trim() || raw;
  switch (command) {
    case '/talk':
      return { ...request, instruction, interaction: { ...request.interaction, intent: 'consult' } };
    case '/ask':
      return { ...request, instruction, interaction: { intent: 'apply', clarification: 'before_apply' } };
    case '/overview':
      return { ...request, instruction, target: { artifact: 'presentation', level: 'deck' } };
    case '/current':
      return { ...request, instruction, target: { ...request.target, level: 'slide' } };
    default:
      return request;
  }
}

export const CommandComposer: React.FC = () => {
  const [text, setText] = useState('');
  const [submitError, setSubmitError] = useState('');
  const [isComposing, setIsComposing] = useState(false);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const { activeProjectId, slidesByProjectId, createProject, finalizePendingNewProject } = useProjectStore();
  const { currentPage } = useDeckStore();
  const { ensureActiveThread } = useThreadStore();
  const { createRun } = useRunStore();
  const { status: runStatus } = useActiveSession();
  const composer = useComposerStore();
  const applyContextDefault = composer.applyContextDefault;
  const focusNonce = composer.focusNonce;
  const disabled = !activeProjectId || runStatus === 'creating' || runStatus === 'running' || runStatus === 'needs_input';

  const slides = activeProjectId && activeProjectId !== 'new-pending' ? slidesByProjectId[activeProjectId] || [] : [];
  const currentSlide = slides[currentPage];

  useEffect(() => {
    applyContextDefault(slides.length > 0);
  }, [applyContextDefault, slides.length]);
  useEffect(() => {
    if (focusNonce > 0) textareaRef.current?.focus();
  }, [focusNonce]);

  const submit = async (deckMaterialization = false) => {
    const raw = text.trim();
    if (disabled || !activeProjectId || (!deckMaterialization && !raw)) return;
    setSubmitError('');
    let projectId = activeProjectId;
    const target = deckMaterialization
      ? { artifact: 'presentation' as const, level: 'deck' as const }
      : {
          artifact: composer.artifact,
          level: composer.level === 'slide' && !currentSlide ? 'deck' as const : composer.level,
          ...(composer.level === 'slide' && currentSlide ? { slide_id: currentSlide.id } : {}),
        };
    let request: CreateRunRequest = {
      target,
      interaction: { intent: composer.intent, clarification: composer.clarification },
      instruction: raw || '基于当前蓝图物化整份 HTML 演示',
    };
    request = applyShortcut(raw, request);
    if (request.target.level === 'slide' && !request.target.slide_id) {
      request.target = { artifact: request.target.artifact, level: 'deck' };
    }

    if (projectId === 'new-pending') {
      try {
        const project = await createProject(raw, raw, 10, 'zh-CN');
        projectId = project.id;
        finalizePendingNewProject(project.id);
        request.target = { artifact: 'blueprint', level: 'deck' };
      } catch (error) {
        setSubmitError(error instanceof Error ? error.message : '创建项目失败，请重试');
        return;
      }
    }
    try {
      const threadId = await ensureActiveThread(projectId);
      const created = await createRun(threadId, request, projectId);
      if (created) setText('');
      else setSubmitError('运行创建失败，请检查时间线中的错误后重试');
    } catch (error) {
      setSubmitError(error instanceof Error ? error.message : '创建会话失败，请重试');
    }
  };

  const handleKeyDown = (event: React.KeyboardEvent) => {
    if (event.nativeEvent.isComposing || isComposing) return;
    if (event.key === 'Enter' && (isMac() ? event.metaKey : event.ctrlKey)) {
      event.preventDefault();
      void submit(false);
    }
  };

  const ring = 'ring-accent/30 border-accent';
  const contextLabel = `本次作用于 ${composer.level === 'slide' ? '当前页' : '整份'}${composer.artifact === 'presentation' ? ' HTML' : '蓝图'} | ${composer.intent === 'consult' ? '仅讨论' : '自动执行'} | ${composer.clarification === 'before_apply' ? '执行前确认' : composer.clarification === 'never' ? '不询问' : '仅阻塞时询问'}`;
  return (
    <div className="border-t border-border bg-background p-4">
      {submitError && (
        <div role="alert" className="mb-2 rounded-md border border-danger/30 bg-danger-soft px-3 py-2 text-xs text-danger">
          {submitError}
        </div>
      )}
      <div className="mb-2 truncate text-[11px] text-text-600" title={contextLabel}>{contextLabel}</div>
      <div className={cn('relative rounded-lg border bg-surface shadow-sm ring-1', ring)}>
        <textarea
          ref={textareaRef}
          value={text}
          onChange={(event) => setText(event.target.value)}
          onKeyDown={handleKeyDown}
          onCompositionStart={() => setIsComposing(true)}
          onCompositionEnd={() => setIsComposing(false)}
          placeholder={composer.intent === 'consult' ? '询问关于当前目标的建议，不会修改项目' : '描述你想修改的内容'}
          disabled={disabled}
          aria-describedby={disabled ? 'composer-disabled-reason' : undefined}
          className="max-h-32 min-h-[60px] w-full resize-none bg-transparent p-3 text-sm text-text-900 placeholder:text-text-400 focus:outline-none disabled:opacity-50"
          rows={2}
        />
        {disabled && activeProjectId && (
          <div id="composer-disabled-reason" className="px-3 pb-1 text-xs text-text-600">
            {runStatus === 'needs_input' ? '请先回答上方问题，或停止当前运行' : '当前运行结束后可继续输入'}
          </div>
        )}
        <div className="flex items-end justify-between gap-2 px-3 pb-2">
          <div className="flex min-w-0 flex-wrap items-center gap-2">
            <ModeSwitcher
              artifact={composer.artifact}
              level={composer.level}
              intent={composer.intent}
              clarification={composer.clarification}
              onArtifactChange={composer.setArtifact}
              onLevelChange={composer.setLevel}
              onIntentChange={composer.setIntent}
              onClarificationChange={composer.setClarification}
              disabled={disabled}
            />
            <button
              type="button"
              onClick={() => void submit(true)}
              disabled={slides.length === 0 || disabled}
              className="inline-flex items-center gap-1 rounded border border-accent/40 px-2 py-1 text-xs font-medium text-accent disabled:opacity-40"
            >
              <WandSparkles className="h-3.5 w-3.5" /> 物化整份
            </button>
          </div>
          <button
            onClick={() => void submit(false)}
            disabled={!text.trim() || disabled}
            className="rounded-md bg-accent p-1.5 text-white disabled:bg-text-400 disabled:opacity-50"
            aria-label="发送"
          >
            <Send className="h-4 w-4" />
          </button>
        </div>
      </div>
    </div>
  );
};
