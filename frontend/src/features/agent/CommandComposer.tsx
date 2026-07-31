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
  const [isComposing, setIsComposing] = useState(false);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const { activeProjectId, slidesByProjectId, createProject, finalizePendingNewProject } = useProjectStore();
  const { currentPage } = useDeckStore();
  const { ensureActiveThread } = useThreadStore();
  const { createRun } = useRunStore();
  const { status: runStatus } = useActiveSession();
  const composer = useComposerStore();
  const disabled = !activeProjectId || runStatus === 'running' || runStatus === 'needs_input';

  const slides = activeProjectId && activeProjectId !== 'new-pending' ? slidesByProjectId[activeProjectId] || [] : [];
  const currentSlide = slides[currentPage];

  useEffect(() => {
    composer.applyContextDefault(slides.length > 0);
  }, [slides.length]);
  useEffect(() => {
    if (composer.focusNonce > 0) textareaRef.current?.focus();
  }, [composer.focusNonce]);

  const submit = async (deckMaterialization = false) => {
    const raw = text.trim();
    if (disabled || !activeProjectId || (!deckMaterialization && !raw)) return;
    let projectId = activeProjectId;
    let target = deckMaterialization
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
        console.error('Failed to create project', error);
        return;
      }
    }
    const threadId = await ensureActiveThread(projectId);
    await createRun(threadId, request);
    setText('');
  };

  const handleKeyDown = (event: React.KeyboardEvent) => {
    if (event.nativeEvent.isComposing || isComposing) return;
    if (event.key === 'Enter' && (isMac() ? event.metaKey : event.ctrlKey)) {
      event.preventDefault();
      void submit(false);
    }
  };

  const ring = composer.artifact === 'blueprint' ? 'ring-mode-outline border-mode-outline' : 'ring-mode-page border-mode-page';
  return (
    <div className="border-t border-border bg-background p-4">
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
          className="max-h-32 min-h-[60px] w-full resize-none bg-transparent p-3 text-sm text-text-900 placeholder:text-text-400 focus:outline-none disabled:opacity-50"
          rows={2}
        />
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
              className="inline-flex items-center gap-1 rounded border border-mode-overview/40 px-2 py-1 text-xs font-medium text-mode-overview disabled:opacity-40"
            >
              <WandSparkles className="h-3.5 w-3.5" /> 物化整份
            </button>
          </div>
          <button
            onClick={() => void submit(false)}
            disabled={!text.trim() || disabled}
            className="rounded-md bg-mode-page p-1.5 text-white disabled:bg-text-400 disabled:opacity-50"
            aria-label="发送"
          >
            <Send className="h-4 w-4" />
          </button>
        </div>
      </div>
    </div>
  );
};
