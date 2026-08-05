import React, { useEffect, useRef, useState } from 'react';
import { Send, StopCircle } from 'lucide-react';
import { llmApi } from '../../api/llm';
import type { CreateRunRequest, LLMProfile } from '../../api/types';
import { useComposerStore } from '../../stores/composerStore';
import { useDeckStore } from '../../stores/deckStore';
import { useProjectStore } from '../../stores/projectStore';
import { useRunStore } from '../../stores/runStore';
import { useThreadStore } from '../../stores/threadStore';
import { isMac } from '../../lib/platform';
import { newClientIdentity } from '../../lib/clientIdentity';
import { InteractionModeButtons } from './InteractionModeButtons';
import { ModelSelector } from './ModelSelector';
import { PlanIndicator } from './PlanIndicator';
import { TargetSelector } from './TargetSelector';
import { useActiveSession } from './useActiveSession';

function applyShortcut(raw: string, request: CreateRunRequest): CreateRunRequest {
  if (!raw.startsWith('/')) return request;
  const [command, ...rest] = raw.split(/\s+/);
  const instruction = rest.join(' ').trim() || raw;
  switch (command) {
    case '/talk':
      return { ...request, instruction, interaction: { intent: 'talk' } };
    case '/ask':
      return { ...request, instruction, interaction: { intent: 'ask' } };
    case '/plan':
      return { ...request, instruction, interaction: { intent: 'plan' } };
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
  const [profiles, setProfiles] = useState<LLMProfile[]>([]);
  const [profilesLoading, setProfilesLoading] = useState(true);
  const [profilesError, setProfilesError] = useState('');
  const { activeProjectId, slidesByProjectId } = useProjectStore();
  const { currentPage } = useDeckStore();
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

  const slides = activeProjectId ? slidesByProjectId[activeProjectId] || [] : [];
  const currentSlide = slides[currentPage];
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
  const submit = async () => {
    const raw = text.trim();
    if (disabled || !activeProjectId || !raw) return;
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
    const target = {
      artifact: composer.artifact,
      level: composer.level === 'slide' && !currentSlide ? 'deck' as const : composer.level,
      ...(composer.level === 'slide' && currentSlide ? { slide_id: currentSlide.id } : {}),
    };
    let request: CreateRunRequest = {
      client_request_id: newClientIdentity('req'),
      model: composer.modelProfileName,
      target,
      interaction: { intent: composer.intent },
      instruction: raw,
    };
    request = applyShortcut(raw, request);
    if (request.target.level === 'slide' && !request.target.slide_id) {
      request.target = { artifact: request.target.artifact, level: 'deck' };
    }
    const selectedProfile = profiles.find((profile) => profile.name === request.model);
    const requiresVision = request.interaction.intent === 'execute' &&
      request.target.artifact === 'presentation';
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

  const requiresVision = composer.intent === 'execute' && composer.artifact === 'presentation';
  const togglePlanIntent = () => {
    composer.setIntent(composer.intent === 'plan' ? 'execute' : 'plan');
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
      {!submitError && profilesError && (
        <div role="alert" className="mb-2 rounded-md border border-danger/30 bg-danger-soft px-3 py-2 text-xs text-danger">
          {profilesError}
        </div>
      )}
      <div className="relative rounded-[22px] border border-border/80 bg-panel shadow-[0_6px_20px_rgba(15,23,42,0.06)]">
        <textarea
          value={text}
          onChange={(event) => setText(event.target.value)}
          onKeyDown={handleKeyDown}
          onCompositionStart={() => setIsComposing(true)}
          onCompositionEnd={() => setIsComposing(false)}
          placeholder={composerPlaceholder}
          disabled={disabled}
          className="max-h-32 min-h-[60px] w-full resize-none bg-transparent p-3 text-sm text-text-900 placeholder:text-text-400 focus:outline-none focus-visible:outline-none focus-visible:ring-0 focus-visible:ring-offset-0 disabled:opacity-50"
          rows={2}
        />
        <div className="flex min-w-0 items-center justify-between gap-1 px-3 pb-2">
          <div className="flex min-w-0 items-center gap-0.5">
            <InteractionModeButtons
              intent={composer.intent}
              onIntentChange={composer.setIntent}
              disabled={disabled || steering}
            />
            <PlanIndicator
              plan={plan}
              running={runActive}
              selected={composer.intent === 'plan'}
              disabled={plan && plan.steps.length > 0 ? false : disabled || steering}
              onSelectPlan={togglePlanIntent}
            />
          </div>
          <div className="flex min-w-0 shrink-0 items-center gap-0.5">
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
                disabled={!text.trim() || disabled || (!steering && (profilesLoading || Boolean(profilesError)))}
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
