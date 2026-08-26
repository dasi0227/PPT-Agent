import React, { useEffect, useLayoutEffect, useRef, useState } from 'react';
import { Send, Sparkles, StopCircle } from 'lucide-react';
import { llmApi } from '../../api/llm';
import { polishApi } from '../../api/polish';
import type { CreateRunRequest, LLMProfile } from '../../api/types';
import { cn } from '../../lib/utils';
import { useComposerStore } from '../../stores/composerStore';
import { useDeckStore } from '../../stores/deckStore';
import { useProjectStore } from '../../stores/projectStore';
import { orderedSlides } from '../deck/selectors';
import { useRunStore } from '../../stores/runStore';
import { useThreadStore } from '../../stores/threadStore';
import { isMac } from '../../lib/platform';
import { newClientIdentity } from '../../lib/clientIdentity';
import { InteractionModeButtons } from './InteractionModeButtons';
import { ModelSelector } from './ModelSelector';
import { PlanIndicator } from './PlanIndicator';
import { TargetSelector } from './TargetSelector';
import { useActiveSession } from './useActiveSession';

const COMPOSER_CONTROLS_FIT_GUARD_PX = 2;
const COMPOSER_CONTROLS_HYSTERESIS_PX = 12;
type ComposerControlsDensity = 'full' | 'left-compact' | 'all-compact';

interface ComposerControlWidths {
  start: number;
  end: number;
  startButtons: number[];
  endButtons: number[];
  gap: number;
}

interface ComposerControlThresholds {
  full: number;
  leftCompact: number;
  allCompact: number;
}

function applyDensityClass(element: HTMLElement, density: ComposerControlsDensity) {
  element.classList.toggle('composer-controls-left-compact', density !== 'full');
  element.classList.toggle('composer-controls-right-compact', density === 'all-compact');
}

function elementWidth(element: HTMLElement): number {
  return Math.ceil(element.getBoundingClientRect().width || element.scrollWidth);
}

function readControlWidths(element: HTMLElement): ComposerControlWidths | null {
  const start = element.querySelector<HTMLElement>('[data-composer-control-group="start"]');
  const end = element.querySelector<HTMLElement>('[data-composer-control-group="end"]');
  if (!start || !end) return null;

  const styles = window.getComputedStyle(element);
  return {
    start: elementWidth(start),
    end: elementWidth(end),
    startButtons: Array.from(start.querySelectorAll<HTMLElement>('.composer-mode-button, .composer-plan-button'))
      .map(elementWidth),
    endButtons: Array.from(end.querySelectorAll<HTMLElement>('.composer-target-button, .composer-model-button'))
      .map(elementWidth),
    gap: Number.parseFloat(styles.columnGap) || 0,
  };
}

function largestExpansion(expanded: number[], compact: number[]): number {
  return expanded.reduce((largest, width, index) => (
    Math.max(largest, width - (compact[index] ?? width))
  ), 0);
}

function measureControlThresholds(bar: HTMLElement): ComposerControlThresholds | null {
  const clone = bar.cloneNode(true) as HTMLElement;
  clone.removeAttribute('data-controls-density');
  clone.classList.add('composer-controls-measure');
  clone.setAttribute('aria-hidden', 'true');
  clone.setAttribute('inert', '');
  clone.querySelectorAll<HTMLElement>('[data-state]').forEach((element) => {
    element.removeAttribute('data-state');
  });
  document.body.appendChild(clone);

  try {
    applyDensityClass(clone, 'full');
    const full = readControlWidths(clone);
    applyDensityClass(clone, 'left-compact');
    const leftCompact = readControlWidths(clone);
    applyDensityClass(clone, 'all-compact');
    const allCompact = readControlWidths(clone);
    if (!full || !leftCompact || !allCompact || full.start <= 0 || full.end <= 0) return null;

    const leftHoverReserve = largestExpansion(full.startButtons, leftCompact.startButtons);
    const rightHoverReserve = largestExpansion(full.endButtons, allCompact.endButtons);
    const guard = COMPOSER_CONTROLS_FIT_GUARD_PX;
    return {
      full: full.start + full.end + full.gap + guard,
      leftCompact: leftCompact.start + full.end + full.gap + leftHoverReserve + guard,
      allCompact: allCompact.start + allCompact.end + full.gap
        + Math.max(leftHoverReserve, rightHoverReserve) + guard,
    };
  } finally {
    clone.remove();
  }
}

function resolveControlsDensity(
  current: ComposerControlsDensity,
  available: number,
  thresholds: ComposerControlThresholds,
): ComposerControlsDensity {
  if (available <= 0) return current;
  const fullThreshold = thresholds.full;
  const allThreshold = Math.min(fullThreshold, thresholds.allCompact);
  const leftThreshold = Math.min(fullThreshold, Math.max(allThreshold, thresholds.leftCompact));

  if (current === 'full') {
    if (available < leftThreshold) return 'all-compact';
    if (available < fullThreshold) return 'left-compact';
    return current;
  }

  if (current === 'left-compact') {
    if (available < leftThreshold) return 'all-compact';
    if (available >= fullThreshold + COMPOSER_CONTROLS_HYSTERESIS_PX) return 'full';
    return current;
  }

  if (available >= fullThreshold + COMPOSER_CONTROLS_HYSTERESIS_PX) return 'full';
  if (available >= leftThreshold + COMPOSER_CONTROLS_HYSTERESIS_PX) return 'left-compact';
  return current;
}

function applyShortcut(raw: string, request: CreateRunRequest): CreateRunRequest {
  if (!raw.startsWith('/')) return request;
  const [command, ...rest] = raw.split(/\s+/);
  const instruction = rest.join(' ').trim() || raw;
  switch (command) {
    case '/talk':
      return { ...request, instruction, mode: 'talk' };
    case '/ask':
      return { ...request, instruction, mode: 'ask' };
    case '/plan':
      return { ...request, instruction, mode: 'plan' };
    case '/overview':
      return { ...request, instruction, scope: { artifact: 'ppt', level: 'deck' } };
    case '/current':
      return { ...request, instruction, scope: { ...request.scope, level: 'slide' } };
    default:
      return request;
  }
}

export const CommandComposer: React.FC = () => {
  const [text, setText] = useState('');
  const [submitError, setSubmitError] = useState('');
  const [isComposing, setIsComposing] = useState(false);
  const [profiles, setProfiles] = useState<LLMProfile[]>([]);
  const [profilesLoading, setProfilesLoading] = useState(true);
  const [profilesError, setProfilesError] = useState('');
  const [controlsDensity, setControlsDensity] = useState<ComposerControlsDensity>('full');
  const [polishing, setPolishing] = useState(false);
  const controlBarRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const polishAbortRef = useRef<AbortController | null>(null);
  const polishRequestRef = useRef(0);
  const { activeProjectId, contentByProjectId } = useProjectStore();
  const { currentSlideId } = useDeckStore();
  const { activeThreadIdByProjectId, ensureActiveThread } = useThreadStore();
  const { cancelRun, createRun, steerRun } = useRunStore();
  const { status: runStatus, activeRunId, plan } = useActiveSession();
  const composer = useComposerStore();
  const applyContextDefault = composer.applyContextDefault;
  const resetForProject = composer.resetForProject;
  const previousProjectId = useRef(activeProjectId);
  const steering = runStatus === 'running' && Boolean(activeRunId);
  const disabled = !activeProjectId || runStatus === 'creating' || runStatus === 'waiting' || runStatus === 'canceling';
  const runActive = runStatus === 'creating' || runStatus === 'running' || runStatus === 'waiting' || runStatus === 'canceling';
  const activeThreadId = activeProjectId ? activeThreadIdByProjectId[activeProjectId] : undefined;
  const showCancelButton = Boolean(activeRunId)
    && (runStatus === 'creating' || runStatus === 'running' || runStatus === 'waiting' || runStatus === 'canceling')
    && text.trim() === '';
  const disabledPlaceholder = runStatus === 'waiting'
    ? '请先回答上方问题'
    : runStatus === 'canceling'
      ? '正在取消当前任务'
      : runStatus === 'creating'
        ? '正在创建任务'
        : '输入你的想法与目标';
  const composerPlaceholder = disabled && activeProjectId
    ? disabledPlaceholder
    : steering
      ? '追加对当前任务的要求'
      : '输入你的想法与目标';

  const slides = orderedSlides(activeProjectId ? contentByProjectId[activeProjectId] : undefined);
  const currentSlide = slides.find((slide) => slide.id === currentSlideId);
  const isEmptyProject = Boolean(activeProjectId) && slides.length === 0;

  useEffect(() => {
    if (previousProjectId.current === activeProjectId) return;
    previousProjectId.current = activeProjectId;
    resetForProject();
  }, [activeProjectId, resetForProject]);
  useEffect(() => {
    applyContextDefault(slides.length > 0);
  }, [activeProjectId, applyContextDefault, slides.length]);
  useEffect(() => {
    let current = true;
    setProfilesLoading(true);
    setProfilesError('');
    void llmApi.profiles()
      .then((response) => {
        if (!current) return;
        setProfiles(response.profiles);
        const remembered = useComposerStore.getState().modelProfileName;
        const selected = response.profiles.some((profile) => profile.name === remembered)
          ? remembered
          : response.default;
        if (selected) useComposerStore.getState().setModelProfileName(selected);
      })
      .catch(() => {
        if (!current) return;
        setProfiles([]);
        setProfilesError('模型列表加载失败，请刷新后重试');
      })
      .finally(() => {
        if (current) setProfilesLoading(false);
      });
    return () => { current = false; };
  }, []);

  useEffect(() => {
    polishRequestRef.current += 1;
    polishAbortRef.current?.abort();
    polishAbortRef.current = null;
    setPolishing(false);
    return () => {
      polishRequestRef.current += 1;
      polishAbortRef.current?.abort();
      polishAbortRef.current = null;
    };
  }, [activeProjectId]);

  useLayoutEffect(() => {
    const bar = controlBarRef.current;
    if (!bar) return undefined;
    let thresholds = measureControlThresholds(bar);

    const updateDensity = () => {
      if (!thresholds) return;
      const currentThresholds = thresholds;
      const styles = window.getComputedStyle(bar);
      const horizontalPadding = (Number.parseFloat(styles.paddingLeft) || 0)
        + (Number.parseFloat(styles.paddingRight) || 0);
      const available = bar.clientWidth - horizontalPadding;
      setControlsDensity((current) => resolveControlsDensity(current, available, currentThresholds));
    };

    const refreshThresholds = () => {
      thresholds = measureControlThresholds(bar) ?? thresholds;
      updateDensity();
    };

    updateDensity();
    const observer = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(updateDensity);
    observer?.observe(bar);
    window.addEventListener('resize', refreshThresholds);
    return () => {
      observer?.disconnect();
      window.removeEventListener('resize', refreshThresholds);
    };
  }, [
    composer.artifact,
    composer.level,
    composer.modelProfileName,
    isEmptyProject,
    profiles.length,
    profilesLoading,
    showCancelButton,
  ]);

  const submit = async () => {
    const raw = text.trim();
    if (disabled || polishing || !activeProjectId || !raw) return;
    setSubmitError('');
    const projectId = activeProjectId;
    let threadId: string;
    try {
      threadId = await ensureActiveThread(projectId);
    } catch (error) {
      setSubmitError(error instanceof Error ? error.message : '创建会话失败，请重试');
      return;
    }
    if (steering && activeRunId) {
      const accepted = await steerRun(threadId, activeRunId, raw, newClientIdentity('msg'));
      if (accepted) setText('');
      else setSubmitError('追加要求未能加入当前任务；文本已保留，可在任务结束后作为新请求发送');
      return;
    }
    if (profilesError || profilesLoading || !composer.modelProfileName) {
      setSubmitError(profilesError || '模型列表仍在加载，请稍候');
      return;
    }
    const scope = {
      artifact: composer.artifact,
      level: composer.level === 'slide' && !currentSlide ? 'deck' as const : composer.level,
      ...(composer.level === 'slide' && currentSlide ? { slide_id: currentSlide.id } : {}),
    };
    let request: CreateRunRequest = {
      client_request_id: newClientIdentity('req'),
      model: composer.modelProfileName,
      scope,
      mode: composer.mode,
      instruction: raw,
    };
    request = applyShortcut(raw, request);
    if (request.scope.level === 'slide' && !request.scope.slide_id) {
      request.scope = { artifact: request.scope.artifact, level: 'deck' };
    }
    const selectedProfile = profiles.find((profile) => profile.name === request.model);
    const requiresVision = request.mode === 'execute' &&
      request.scope.artifact === 'ppt';
    if (!selectedProfile) {
      setSubmitError('所选模型已不可用，请重新选择');
      return;
    }
    if (requiresVision && !selectedProfile.capabilities.vision) {
      setSubmitError('当前任务需要页面图片观察，请选择支持页面观察的模型');
      return;
    }

    try {
      const created = await createRun(threadId, request, projectId);
      if (created) setText('');
      else setSubmitError('运行创建失败，请检查时间线中的错误后重试');
    } catch (error) {
      setSubmitError(error instanceof Error ? error.message : '创建会话失败，请重试');
    }
  };

  const cancelActiveRun = async () => {
    if (!activeThreadId || !activeRunId || runStatus === 'canceling') return;
    setSubmitError('');
    await cancelRun(activeThreadId, activeRunId);
  };

  const requiresVision = composer.mode === 'execute' && composer.artifact === 'ppt';
  const togglePlanIntent = () => {
    composer.setIntent(composer.mode === 'plan' ? 'execute' : 'plan');
  };

  const polishText = async () => {
    const textarea = textareaRef.current;
    const instruction = text.trim();
    if (!textarea || disabled || polishing || !activeProjectId || !instruction) return;
    if (profilesError || profilesLoading || !composer.modelProfileName) {
      setSubmitError(profilesError || '模型列表仍在加载，请稍候');
      return;
    }
    const selectionStart = textarea.selectionStart;
    const selectionEnd = textarea.selectionEnd;
    const requestID = polishRequestRef.current + 1;
    polishRequestRef.current = requestID;
    const controller = new AbortController();
    polishAbortRef.current?.abort();
    polishAbortRef.current = controller;
    setSubmitError('');
    setPolishing(true);
    const scope = {
      artifact: composer.artifact,
      level: composer.level === 'slide' && !currentSlide ? 'deck' as const : composer.level,
      ...(composer.level === 'slide' && currentSlide ? { slide_id: currentSlide.id } : {}),
    };
    try {
      const result = await polishApi.polish(activeProjectId, {
        instruction,
        ...(activeThreadId ? { thread_id: activeThreadId } : {}),
        scope,
        mode: composer.mode,
        model: composer.modelProfileName,
      }, controller.signal);
      if (polishRequestRef.current !== requestID || controller.signal.aborted) return;
      setText(result.polished_instruction);
      requestAnimationFrame(() => {
        const current = textareaRef.current;
        if (!current || current.disabled) return;
        current.focus();
        const end = result.polished_instruction.length;
        current.setSelectionRange(end, end);
      });
    } catch (error) {
      if (polishRequestRef.current !== requestID || controller.signal.aborted) return;
      setSubmitError(error instanceof Error ? error.message : '润色失败，请重试');
      requestAnimationFrame(() => {
        const current = textareaRef.current;
        if (!current || current.disabled) return;
        current.focus();
        current.setSelectionRange(selectionStart, selectionEnd);
      });
    } finally {
      if (polishRequestRef.current === requestID) {
        polishAbortRef.current = null;
        setPolishing(false);
      }
    }
  };

  const handleKeyDown = (event: React.KeyboardEvent) => {
    if (event.nativeEvent.isComposing || isComposing || polishing) return;
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
      {!submitError && profilesError && (
        <div role="alert" className="mb-2 rounded-md border border-danger/30 bg-danger-soft px-3 py-2 text-xs text-danger">
          {profilesError}
        </div>
      )}
      <div className="relative rounded-[22px] border border-border/80 bg-panel shadow-[0_6px_20px_rgba(15,23,42,0.06)]">
        <div className="relative overflow-hidden rounded-t-[22px]">
          <textarea
            ref={textareaRef}
            value={text}
            onChange={(event) => setText(event.target.value)}
            onKeyDown={handleKeyDown}
            onCompositionStart={() => setIsComposing(true)}
            onCompositionEnd={() => setIsComposing(false)}
            placeholder={composerPlaceholder}
            disabled={disabled}
            readOnly={polishing}
            aria-busy={polishing}
            className="max-h-32 min-h-[60px] w-full resize-none bg-transparent py-3 pl-3 pr-12 text-sm text-text-900 placeholder:text-text-400 focus:outline-none focus-visible:outline-none focus-visible:ring-0 focus-visible:ring-offset-0 disabled:opacity-50"
            rows={2}
          />
          {text.trim() !== '' && !disabled && (
            <button
              type="button"
              onClick={() => void polishText()}
              disabled={polishing}
              aria-label={polishing ? '正在润色表达' : '润色表达'}
              title={polishing ? '正在润色表达' : '润色表达'}
              className="absolute right-2.5 top-2.5 z-10 inline-flex h-7 w-7 items-center justify-center rounded-lg border border-border/80 bg-surface/90 text-accent shadow-sm backdrop-blur-sm hover:bg-accent-soft disabled:cursor-wait disabled:opacity-100"
            >
              <Sparkles
                className={cn('h-3.5 w-3.5', polishing && 'polish-sparkles-active')}
                strokeWidth={1.75}
              />
            </button>
          )}
          {polishing && <span className="composer-polish-sweep" aria-hidden="true" />}
          <span className="sr-only" aria-live="polite">
            {polishing ? '正在润色表达' : ''}
          </span>
        </div>
        <div
          ref={controlBarRef}
          data-controls-density={controlsDensity}
          className={cn(
            'composer-control-bar flex min-w-0 items-center justify-between gap-3 px-3 pb-2',
            controlsDensity !== 'full' && 'composer-controls-left-compact',
            controlsDensity === 'all-compact' && 'composer-controls-right-compact',
          )}
        >
          <div data-composer-control-group="start" className="flex min-w-0 items-center gap-0.5">
            <InteractionModeButtons
              mode={composer.mode}
              onIntentChange={composer.setIntent}
              disabled={disabled || steering}
            />
            <PlanIndicator
              plan={plan}
              running={runActive}
              selected={composer.mode === 'plan'}
              disabled={plan && plan.steps.length > 0 ? false : disabled || steering}
              onSelectPlan={togglePlanIntent}
            />
          </div>
          <div data-composer-control-group="end" className="flex min-w-0 shrink-0 items-center gap-0.5">
            <TargetSelector
              artifact={composer.artifact}
              level={composer.level}
              onTargetChange={(target) => {
                composer.setArtifact(target.artifact);
                composer.setLevel(target.level);
              }}
              disabled={disabled || steering}
              locked={isEmptyProject}
            />
            <ModelSelector
              profiles={profiles}
              value={composer.modelProfileName}
              requiresVision={requiresVision}
              loading={profilesLoading}
              disabled={disabled || steering}
              onChange={composer.setModelProfileName}
            />
            {showCancelButton ? (
              <button
                onClick={() => void cancelActiveRun()}
                disabled={!activeThreadId || runStatus === 'canceling'}
                className="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-md border border-border bg-surface text-danger transition-colors hover:bg-danger-soft disabled:bg-panel-muted disabled:text-text-400 disabled:opacity-60"
                aria-label={runStatus === 'canceling' ? '正在取消' : '终止运行'}
                title={runStatus === 'canceling' ? '正在取消' : '终止运行'}
              >
                <StopCircle className="h-4 w-4" strokeWidth={1.75} />
              </button>
            ) : (
              <button
                onClick={() => void submit()}
                disabled={!text.trim() || disabled || polishing || (!steering && (profilesLoading || Boolean(profilesError)))}
                className="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-accent text-white disabled:bg-text-400 disabled:opacity-50"
                aria-label="发送"
              >
                <Send className="h-4 w-4" />
              </button>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};
