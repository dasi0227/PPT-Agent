import React, { useEffect, useRef, useState } from 'react';
import { Send } from 'lucide-react';
import type { CreateRunRequest } from '../../api/types';
import { useComposerStore } from '../../stores/composerStore';
import { useDeckStore } from '../../stores/deckStore';
import { useProjectStore } from '../../stores/projectStore';
import { useRunStore } from '../../stores/runStore';
import { useThreadStore } from '../../stores/threadStore';
import { isMac } from '../../lib/platform';
import { InteractionModeButtons } from './InteractionModeButtons';
import { TargetSelector } from './TargetSelector';
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
  const { activeProjectId, slidesByProjectId } = useProjectStore();
  const { currentPage } = useDeckStore();
  const { ensureActiveThread } = useThreadStore();
  const { createRun } = useRunStore();
  const { status: runStatus } = useActiveSession();
  const composer = useComposerStore();
  const applyContextDefault = composer.applyContextDefault;
  const resetForProject = composer.resetForProject;
  const previousProjectId = useRef(activeProjectId);
  const disabled = !activeProjectId || runStatus === 'creating' || runStatus === 'running' || runStatus === 'needs_input';

  const slides = activeProjectId ? slidesByProjectId[activeProjectId] || [] : [];
  const currentSlide = slides[currentPage];

  useEffect(() => {
    if (previousProjectId.current === activeProjectId) return;
    previousProjectId.current = activeProjectId;
    resetForProject();
  }, [activeProjectId, resetForProject]);
  useEffect(() => {
    applyContextDefault(slides.length > 0);
  }, [activeProjectId, applyContextDefault, slides.length]);
  const submit = async () => {
    const raw = text.trim();
    if (disabled || !activeProjectId || !raw) return;
    setSubmitError('');
    const projectId = activeProjectId;
    const target = {
      artifact: composer.artifact,
      level: composer.level === 'slide' && !currentSlide ? 'deck' as const : composer.level,
      ...(composer.level === 'slide' && currentSlide ? { slide_id: currentSlide.id } : {}),
    };
    let request: CreateRunRequest = {
      target,
      interaction: { intent: composer.intent, clarification: composer.clarification },
      instruction: raw,
    };
    request = applyShortcut(raw, request);
    if (request.target.level === 'slide' && !request.target.slide_id) {
      request.target = { artifact: request.target.artifact, level: 'deck' };
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
      void submit();
    }
  };

  return (
    <div className="bg-panel px-3 pb-3 pt-1">
      {submitError && (
        <div role="alert" className="mb-2 rounded-md border border-danger/30 bg-danger-soft px-3 py-2 text-xs text-danger">
          {submitError}
        </div>
      )}
      <div className="relative rounded-[22px] border border-border/80 bg-panel shadow-[0_6px_20px_rgba(15,23,42,0.06)]">
        <textarea
          value={text}
          onChange={(event) => setText(event.target.value)}
          onKeyDown={handleKeyDown}
          onCompositionStart={() => setIsComposing(true)}
          onCompositionEnd={() => setIsComposing(false)}
          placeholder="输入你的想法与目标"
          disabled={disabled}
          aria-describedby={disabled ? 'composer-disabled-reason' : undefined}
          className="max-h-32 min-h-[60px] w-full resize-none bg-transparent p-3 text-sm text-text-900 placeholder:text-text-400 focus:outline-none focus-visible:outline-none focus-visible:ring-0 focus-visible:ring-offset-0 disabled:opacity-50"
          rows={2}
        />
        {disabled && activeProjectId && (
          <div id="composer-disabled-reason" className="px-3 pb-1 text-xs text-text-600">
            {runStatus === 'needs_input' ? '请先回答上方问题，或停止当前运行' : '当前运行结束后可继续输入'}
          </div>
        )}
        <div className="flex min-w-0 items-center justify-between gap-1 px-3 pb-2">
          <InteractionModeButtons
            intent={composer.intent}
            clarification={composer.clarification}
            onIntentChange={composer.setIntent}
            onClarificationChange={composer.setClarification}
            disabled={disabled}
          />
          <div className="flex min-w-0 shrink-0 items-center gap-0.5">
            <TargetSelector
              artifact={composer.artifact}
              level={composer.level}
              onTargetChange={(target) => {
                composer.setArtifact(target.artifact);
                composer.setLevel(target.level);
              }}
              disabled={disabled}
            />
            <button
              onClick={() => void submit()}
              disabled={!text.trim() || disabled}
              className="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-accent text-white disabled:bg-text-400 disabled:opacity-50"
              aria-label="发送"
            >
              <Send className="h-4 w-4" />
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};
