import React, { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { FileImage, Paperclip, Send, Sparkles, StopCircle, X } from 'lucide-react';
import { attachmentsApi } from '../../api/attachments';
import { llmApi } from '../../api/llm';
import { polishApi } from '../../api/polish';
import { skillsApi } from '../../api/skills';
import type { CreateRunRequest, CreateRunScopeInput, LLMProfile, Skill } from '../../api/types';
import { cn } from '../../lib/utils';
import { MAX_MESSAGE_ATTACHMENTS, type ComposerAttachment, useComposerStore } from '../../stores/composerStore';
import { useDeckStore } from '../../stores/deckStore';
import { useProjectStore } from '../../stores/projectStore';
import { orderedSlides } from '../deck/selectors';
import { useRunStore } from '../../stores/runStore';
import { useThreadStore } from '../../stores/threadStore';
import { isMac } from '../../lib/platform';
import { newClientIdentity } from '../../lib/clientIdentity';
import { InteractionModeButtons } from './InteractionModeButtons';
import { ModelSelector } from './ModelSelector';
import { SkillSelector } from './SkillSelector';
import { PlanIndicator } from './PlanIndicator';
import { TargetSelector } from './TargetSelector';
import { useActiveSession } from './useActiveSession';
import { useGitCommitStore } from '../../stores/gitCommitStore';
import { useBriefingStore } from '../../stores/briefingStore';
import {
  PromptComposerEditor,
  type PromptComposerEditorHandle,
  type SlashMenuOption,
} from './PromptComposerEditor';
import { resolveSlashCommands, type SlashCommandId } from './promptMatching';

function composerScopeInput(
  composer: ReturnType<typeof useComposerStore.getState>,
  currentSlideId: string | undefined,
): CreateRunScopeInput | null {
  if (composer.scopeObject === 'global') {
    return { object: 'global', selection: { kind: 'all_pages' } };
  }
  if (composer.scopeSelection === 'current_page') {
    return currentSlideId
      ? { object: composer.scopeObject, selection: { kind: 'current_page', current_slide_id: currentSlideId } }
      : { object: composer.scopeObject, selection: { kind: 'all_pages' } };
  }
  if (composer.scopeSelection === 'custom_pages') {
    return composer.customSlideIds.length > 0
      ? { object: composer.scopeObject, selection: { kind: 'custom_pages', slide_ids: composer.customSlideIds } }
      : null;
  }
  if (composer.scopeSelection === 'custom_sections') {
    return composer.customSectionIds.length > 0
      ? { object: composer.scopeObject, selection: { kind: 'custom_sections', section_ids: composer.customSectionIds } }
      : null;
  }
  return { object: composer.scopeObject, selection: { kind: 'all_pages' } };
}

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
    endButtons: Array.from(end.querySelectorAll<HTMLElement>('.composer-skill-button, .composer-target-button, .composer-model-button'))
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

function formatFileSize(size: number): string {
	if (size >= 1024 * 1024) return `${(size / (1024 * 1024)).toFixed(1)} MiB`;
	if (size >= 1024) return `${Math.max(1, Math.round(size / 1024))} KiB`;
	return `${size} B`;
}

function supportedImageFile(file: File): boolean {
	return ['image/png', 'image/jpeg', 'image/webp'].includes(file.type);
}

export const CommandComposer: React.FC = () => {
  const [text, setText] = useState('');
  const [submitError, setSubmitError] = useState('');
	const [uploadingCount, setUploadingCount] = useState(0);
  const [isComposing, setIsComposing] = useState(false);
  const [profiles, setProfiles] = useState<LLMProfile[]>([]);
  const [profilesLoading, setProfilesLoading] = useState(true);
  const [profilesError, setProfilesError] = useState('');
  const [skills, setSkills] = useState<Skill[]>([]);
  const [skillsLoading, setSkillsLoading] = useState(true);
  const [skillsError, setSkillsError] = useState('');
  const [controlsDensity, setControlsDensity] = useState<ComposerControlsDensity>('full');
  const controlBarRef = useRef<HTMLDivElement>(null);
  const editorRef = useRef<PromptComposerEditorHandle>(null);
	const fileInputRef = useRef<HTMLInputElement>(null);
  const polishAbortRef = useRef<AbortController | null>(null);
  const polishRequestRef = useRef(0);
  const { activeProjectId, contentByProjectId } = useProjectStore();
  const { currentSlideId } = useDeckStore();
  const { activeThreadIdByProjectId, ensureActiveThread } = useThreadStore();
  const { cancelRun, createRun, steerRun } = useRunStore();
  const { status: runStatus, activeRunId, plan } = useActiveSession();
  const commitSession = useGitCommitStore((state) => (
    activeProjectId ? state.sessions[activeProjectId] : undefined
  ));
  const startCommit = useGitCommitStore((state) => state.start);
  const briefingSession = useBriefingStore((state) => (
    activeProjectId ? state.sessions[activeProjectId] : undefined
  ));
  const generateBriefing = useBriefingStore((state) => state.generate);
  const briefingActive = briefingSession?.status === 'generating';
  const commitActive = commitSession?.status === 'creating' || commitSession?.status === 'running';
  const composer = useComposerStore();
  const polishing = composer.polishing;
  const setPolishing = composer.setPolishing;
  const applyContextDefault = composer.applyContextDefault;
  const resetForProject = composer.resetForProject;
  const previousProjectId = useRef(activeProjectId);
  const steering = runStatus === 'running' && Boolean(activeRunId);
  const disabled = !activeProjectId || runStatus === 'creating' || runStatus === 'waiting' || runStatus === 'recovering' || runStatus === 'canceling';
  const runActive = runStatus === 'creating' || runStatus === 'running' || runStatus === 'waiting' || runStatus === 'paused' || runStatus === 'recovering' || runStatus === 'canceling';
  const activeThreadId = activeProjectId ? activeThreadIdByProjectId[activeProjectId] : undefined;
  const activeThreadDraft = activeThreadId ? composer.threadDrafts[activeThreadId] : undefined;
	const activeAttachments = activeThreadId ? composer.threadAttachments[activeThreadId] ?? [] : [];
	const hasAttachments = activeAttachments.length > 0;
	const hasPendingUploads = uploadingCount > 0;
  const setComposerText = (nextText: string) => {
    setText(nextText);
    if (activeThreadId) composer.setThreadDraft(activeThreadId, nextText);
  };
  const showCancelButton = Boolean(activeRunId)
    && (runStatus === 'creating' || runStatus === 'running' || runStatus === 'waiting' || runStatus === 'recovering' || runStatus === 'canceling')
	&& text.trim() === '' && !hasAttachments;
  const disabledPlaceholder = runStatus === 'waiting'
    ? '请先回答上方问题'
    : runStatus === 'recovering'
      ? '正在恢复当前任务'
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
	const requiresVision = hasAttachments || (composer.mode === 'execute' && ['html', 'presentation', 'global'].includes(composer.scopeObject));

  const activeSnapshot = activeProjectId ? contentByProjectId[activeProjectId] : undefined;
  const slides = useMemo(() => orderedSlides(activeSnapshot), [activeSnapshot]);
  const pageCandidates = useMemo(() => slides.map((slide, index) => ({
    slideId: slide.id,
    ordinal: index + 1,
    title: slide.title,
    keyMessage: slide.spec?.key_message ?? '',
    specState: slide.spec ? 'ready' as const : 'pending' as const,
    htmlState: slide.materialization?.state ?? 'not_materialized' as const,
  })), [slides]);
  const scopePages = useMemo(() => pageCandidates.map((page) => ({ id: page.slideId, ordinal: page.ordinal, title: page.title })), [pageCandidates]);
  const scopeSections = useMemo(() => (activeSnapshot?.outline.sections ?? []).map((section) => ({
    id: section.id,
    title: section.title,
    pageCount: section.slides.length + section.subsections.reduce((total, subsection) => total + subsection.slides.length, 0),
  })), [activeSnapshot]);
  const currentSlide = slides.find((slide) => slide.id === currentSlideId);
  const isEmptyProject = Boolean(activeProjectId) && slides.length === 0;
  const scopeSelectionEmpty = (composer.scopeSelection === 'custom_pages' && composer.customSlideIds.length === 0)
    || (composer.scopeSelection === 'custom_sections' && composer.customSectionIds.length === 0);
  const slashCommands = useMemo(() => resolveSlashCommands({
    runActive,
    emptyProject: isEmptyProject,
    operationBusy: commitActive || polishing || briefingActive,
    hasPolishText: text.replace(/(?:^|[ \n])\/[^ \n/]*$/, '').trim().length > 0,
  }), [briefingActive, commitActive, isEmptyProject, polishing, runActive, text]);
  const modelOptions = useMemo<SlashMenuOption[]>(() => profiles.map((profile) => ({
    id: profile.name,
    label: profile.name,
    description: profile.model,
    selected: profile.name === composer.modelProfileName,
    disabled: requiresVision && !profile.capabilities.vision,
  })), [composer.modelProfileName, profiles, requiresVision]);
  const targetOptions = useMemo<SlashMenuOption[]>(() => [
    { id: 'object:spec', label: '设计稿', selected: composer.scopeObject === 'spec' },
    { id: 'object:html', label: '幻灯片', selected: composer.scopeObject === 'html' },
    { id: 'object:presentation', label: '演示文稿', selected: composer.scopeObject === 'presentation' },
    { id: 'object:global', label: '全局资源', selected: composer.scopeObject === 'global' },
  ], [composer.scopeObject]);

  useEffect(() => {
    if (previousProjectId.current === activeProjectId) return;
    previousProjectId.current = activeProjectId;
    setText('');
    editorRef.current?.setPlainText('');
    resetForProject();
  }, [activeProjectId, resetForProject]);
  useEffect(() => {
    const draft = activeThreadDraft ?? '';
    setText(draft);
    editorRef.current?.setPlainText(draft);
  }, [activeThreadId, activeThreadDraft]);
  useEffect(() => {
    applyContextDefault(slides.length > 0);
  }, [activeProjectId, applyContextDefault, slides.length]);
  useEffect(() => {
    composer.reconcileScopeIds(scopePages.map((page) => page.id), scopeSections.map((section) => section.id));
  }, [composer.reconcileScopeIds, scopePages, scopeSections]);
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
    let current = true;
    setSkillsLoading(true);
    setSkillsError('');
    void skillsApi.list()
      .then((response) => {
        if (!current) return;
        const enabledSkills = response.skills.filter((skill) => !skill.disabled);
        setSkills(enabledSkills);
        useComposerStore.getState().reconcileSkills(enabledSkills.map((skill) => skill.id));
      })
      .catch(() => {
        if (!current) return;
        setSkills([]);
        setSkillsError('技能列表加载失败');
        useComposerStore.getState().reconcileSkills([]);
      })
      .finally(() => {
        if (current) setSkillsLoading(false);
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
  }, [activeProjectId, setPolishing]);

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
    composer.scopeObject,
    composer.scopeSelection,
    composer.modelProfileName,
    composer.selectedSkillIds.length,
    isEmptyProject,
    profiles.length,
    profilesLoading,
    skills.length,
    skillsLoading,
    showCancelButton,
		hasAttachments,
  ]);

	const uploadFiles = async (files: File[]) => {
		if (!activeProjectId || disabled || files.length === 0) return;
		const unsupported = files.find((file) => !supportedImageFile(file));
		if (unsupported) {
			setSubmitError('仅支持 PNG、JPG 和 WebP 图片');
			return;
		}
		const oversized = files.find((file) => file.size > 10 * 1024 * 1024);
		if (oversized) {
			setSubmitError('图片超过 10 MiB，请压缩后重试');
			return;
		}
		if (activeAttachments.length + uploadingCount + files.length > MAX_MESSAGE_ATTACHMENTS) {
			setSubmitError('每条消息最多添加 8 张图片');
			return;
		}
		setSubmitError('');
		let threadId: string;
		try {
			threadId = activeThreadId ?? await ensureActiveThread(activeProjectId);
		} catch (error) {
			setSubmitError(error instanceof Error ? error.message : '创建会话失败，请重试');
			return;
		}
		setUploadingCount((count) => count + files.length);
		const results = await Promise.all(files.map(async (file) => {
			try {
				const uploaded = await attachmentsApi.upload(activeProjectId, file);
				const item: ComposerAttachment = {
					attachmentId: uploaded.id, name: uploaded.original_name,
					size: uploaded.size_bytes, mediaType: uploaded.media_type,
				};
				composer.addThreadAttachment(threadId, item);
				return null;
			} catch (error) {
				return error instanceof Error ? error.message : `${file.name} 上传失败，请重试`;
			}
		}));
		setUploadingCount((count) => Math.max(0, count - files.length));
		const failure = results.find((result): result is string => Boolean(result));
		if (failure) setSubmitError(failure);
	};

  const submit = async () => {
    const editor = editorRef.current;
    const raw = (editor?.getSubmitText() ?? text).trim();
    const componentNames = editor?.getComponentNames() ?? [];
    const mentionedSlideIds = editor?.getMentionedSlideIds() ?? [];
	if (disabled || commitActive || polishing || briefingActive || !activeProjectId || hasPendingUploads || (!raw && !hasAttachments)) return;
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
		const accepted = await steerRun(threadId, activeRunId, raw, newClientIdentity('msg'), activeAttachments.map((attachment) => attachment.attachmentId));
      if (accepted) {
        setText('');
        editorRef.current?.setPlainText('');
		composer.clearThreadDraft(threadId);
      }
      else setSubmitError('追加要求未能加入当前任务；文本已保留，可在任务结束后作为新请求发送');
      return;
    }
    if (profilesError || profilesLoading || !composer.modelProfileName) {
      setSubmitError(profilesError || '模型列表仍在加载，请稍候');
      return;
    }
    const scope = composerScopeInput(composer, currentSlide?.id);
    if (!scope) {
      setSubmitError(composer.scopeSelection === 'custom_pages' ? '请至少选择一页' : '请至少选择一章');
      return;
    }
    const request: CreateRunRequest = {
      client_request_id: newClientIdentity('req'),
      model: composer.modelProfileName,
      scope,
      mode: composer.mode,
      instruction: raw,
		...(hasAttachments ? { attachment_ids: activeAttachments.map((attachment) => attachment.attachmentId) } : {}),
      ...(composer.selectedSkillIds.length > 0 ? { skill_ids: composer.selectedSkillIds } : {}),
      ...(componentNames.length > 0 ? { component_names: componentNames } : {}),
      ...(mentionedSlideIds.length > 0 ? { mentioned_slide_ids: mentionedSlideIds } : {}),
    };
    const selectedProfile = profiles.find((profile) => profile.name === request.model);
    const requiresVision = hasAttachments || (request.mode === 'execute' &&
      ['html', 'presentation', 'global'].includes(request.scope.object));
    if (!selectedProfile) {
      setSubmitError('所选模型已不可用，请重新选择');
      return;
    }
    if (requiresVision && !selectedProfile.capabilities.vision) {
      setSubmitError('当前任务需要页面图片观察，请选择支持页面观察的模型');
      return;
    }

    try {
      if (runStatus === 'paused' && activeRunId) {
        const ended = await cancelRun(threadId, activeRunId, 'superseded');
        if (!ended) {
          setSubmitError('此前任务未能结束，暂时无法发送新消息');
          return;
        }
      }
      const created = await createRun(threadId, request, projectId);
      if (created) {
        setText('');
        editorRef.current?.setPlainText('');
		composer.clearThreadDraft(threadId);
      }
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

  const togglePlanIntent = () => {
    composer.setIntent(composer.mode === 'plan' ? 'execute' : 'plan');
  };

  const polishText = async () => {
    const editor = editorRef.current;
    const instruction = editor?.getPlainText().trim() ?? '';
    if (!editor || disabled || commitActive || polishing || briefingActive || !activeProjectId || !instruction) return;
    if (profilesError || profilesLoading || !composer.modelProfileName) {
      setSubmitError(profilesError || '模型列表仍在加载，请稍候');
      return;
    }
    const selection = editor.captureSelection();
    const requestID = polishRequestRef.current + 1;
    polishRequestRef.current = requestID;
    const controller = new AbortController();
    polishAbortRef.current?.abort();
    polishAbortRef.current = controller;
    setSubmitError('');
    setPolishing(true);
    const scope = composerScopeInput(composer, currentSlide?.id);
    if (!scope) {
      setSubmitError(composer.scopeSelection === 'custom_pages' ? '请至少选择一页' : '请至少选择一章');
      return;
    }
    try {
      const result = await polishApi.polish(activeProjectId, {
        instruction,
        ...(activeThreadId ? { thread_id: activeThreadId } : {}),
        scope,
        mode: composer.mode,
        model: composer.modelProfileName,
      }, controller.signal);
      if (polishRequestRef.current !== requestID || controller.signal.aborted) return;
      setComposerText(result.polished_instruction);
      requestAnimationFrame(() => {
        editorRef.current?.setPlainText(result.polished_instruction);
        editorRef.current?.focusEnd();
      });
    } catch (error) {
      if (polishRequestRef.current !== requestID || controller.signal.aborted) return;
      setSubmitError(error instanceof Error ? error.message : '润色失败，请重试');
      requestAnimationFrame(() => {
        editorRef.current?.restoreSelection(selection);
      });
    } finally {
      if (polishRequestRef.current === requestID) {
        polishAbortRef.current = null;
        setPolishing(false);
      }
    }
  };

  const executeSlashCommand = async (command: SlashCommandId) => {
    setSubmitError('');
    if (command === 'plan' || command === 'grill' || command === 'chat') {
      composer.setIntent(command);
      return;
    }
    if (command === 'polish') {
      await polishText();
      return;
    }
    if (!activeProjectId || !composer.modelProfileName) return;
    let threadId: string;
    try {
      threadId = await ensureActiveThread(activeProjectId);
    } catch (error) {
      setSubmitError(error instanceof Error ? error.message : '创建会话失败，请重试');
      return;
    }
    if (command === 'commit') {
      await startCommit(activeProjectId, threadId, composer.modelProfileName);
      return;
    }
    if (command === 'kickoff' || command === 'handoff') {
      await generateBriefing(activeProjectId, threadId, composer.modelProfileName, command);
    }
  };

  const selectTargetOption = (id: string) => {
    const [kind, object] = id.split(':');
    if (kind !== 'object' || !['spec', 'html', 'presentation', 'global'].includes(object)) return;
    composer.setScopeObject(object as 'spec' | 'html' | 'presentation' | 'global');
  };

  const handleKeyDown = (event: React.KeyboardEvent) => {
    if (event.nativeEvent.isComposing || isComposing || polishing || commitActive || briefingActive) return;
    if (event.key === 'Enter' && (isMac() ? event.metaKey : event.ctrlKey)) {
      event.preventDefault();
      void submit();
    }
  };
	const selectedProfile = profiles.find((profile) => profile.name === composer.modelProfileName);
	const attachmentModelSupported = !hasAttachments || Boolean(selectedProfile?.capabilities.vision);

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
      {!submitError && !profilesError && skillsError && (
        <div role="alert" className="mb-2 rounded-md border border-warning/30 bg-warning-soft px-3 py-2 text-xs text-warning">
          {skillsError}
        </div>
      )}
      <div className="relative rounded-[22px] border border-border/80 bg-panel shadow-[0_6px_20px_rgba(15,23,42,0.06)]">
		<input
			ref={fileInputRef}
			type="file"
			accept="image/png,image/jpeg,image/webp"
			multiple
			className="sr-only"
			onChange={(event) => {
				const files = Array.from(event.currentTarget.files ?? []);
				event.currentTarget.value = '';
				void uploadFiles(files);
			}}
		/>
        <div className="relative rounded-t-[22px]">
			{hasAttachments && (
				<div className="flex gap-2 overflow-x-auto px-3 pb-1.5 pt-3" aria-label="当前消息引用的图片">
					{activeAttachments.map((attachment) => (
						<div key={attachment.attachmentId} className="relative grid w-44 shrink-0 grid-cols-[38px_minmax(0,1fr)] items-center gap-2 rounded-lg border border-border bg-surface p-1.5 pr-7 shadow-sm">
							<span className="grid h-[38px] w-[38px] place-items-center rounded-md bg-panel-muted text-text-600" aria-hidden="true">
								<FileImage className="h-5 w-5" strokeWidth={1.75} />
							</span>
							<span className="min-w-0">
								<span className="block truncate text-[11px] font-semibold leading-4 text-text-900">{attachment.name}</span>
								<span className="mt-0.5 block text-[11px] leading-3 text-text-600">{formatFileSize(attachment.size)}</span>
							</span>
							<button
								type="button"
								onClick={() => activeThreadId && composer.removeThreadAttachment(activeThreadId, attachment.attachmentId)}
								className="absolute right-1 top-1 inline-flex h-5 w-5 items-center justify-center rounded-md text-text-600 hover:bg-panel-muted hover:text-text-900"
								aria-label={`移除 ${attachment.name}`}
								title="从当前消息移除"
							>
								<X className="h-3.5 w-3.5" strokeWidth={1.75} />
							</button>
						</div>
					))}
				</div>
			)}
          <PromptComposerEditor
            ref={editorRef}
            value={text}
            onChange={setComposerText}
            onKeyDown={handleKeyDown}
            onCompositionChange={setIsComposing}
            placeholder={composerPlaceholder}
            disabled={disabled}
            readOnly={polishing}
            pages={steering ? [] : pageCandidates}
            slashCommands={slashCommands}
            modelOptions={modelOptions}
            targetOptions={targetOptions}
            onSlashCommand={(command) => { void executeSlashCommand(command); }}
            onModelOption={composer.setModelProfileName}
            onTargetOption={selectTargetOption}
			onPasteFiles={(files) => { void uploadFiles(files); }}
          />
          {text.trim() !== '' && !disabled && (
            <button
              type="button"
              onClick={() => void polishText()}
              disabled={polishing || commitActive || briefingActive}
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
			{polishing ? '正在润色表达' : hasPendingUploads ? '正在上传图片' : hasAttachments ? `已添加 ${activeAttachments.length} 张图片` : ''}
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
			<button
				type="button"
				onClick={() => fileInputRef.current?.click()}
				disabled={disabled}
				className="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-md text-text-600 hover:bg-panel-muted hover:text-text-900 disabled:cursor-not-allowed disabled:opacity-50"
				aria-label="选择图片"
				title="选择图片"
			>
				<Paperclip className="h-4 w-4" strokeWidth={1.75} />
			</button>
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
            <SkillSelector
              skills={skills}
              selectedIds={composer.selectedSkillIds}
              loading={skillsLoading}
              disabled={disabled || steering}
              onToggle={composer.toggleSkill}
            />
            <TargetSelector
              object={composer.scopeObject}
              selection={composer.scopeSelection}
              selectedSlideIds={composer.customSlideIds}
              selectedSectionIds={composer.customSectionIds}
              pages={scopePages}
              sections={scopeSections}
              onObjectChange={composer.setScopeObject}
              onSelectionChange={composer.setScopeSelection}
              onToggleSlide={composer.toggleCustomSlide}
              onToggleSection={composer.toggleCustomSection}
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
                disabled={(!text.trim() && !hasAttachments) || hasPendingUploads || !attachmentModelSupported || disabled || commitActive || polishing || briefingActive || (!steering && (scopeSelectionEmpty || profilesLoading || Boolean(profilesError)))}
                className="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-accent text-white disabled:bg-text-400 disabled:opacity-50"
                aria-label={scopeSelectionEmpty ? '请至少选择一页或一章' : !attachmentModelSupported ? '当前模型不支持图片，请更换模型后发送' : '发送'}
                title={scopeSelectionEmpty ? '请至少选择一页或一章' : !attachmentModelSupported ? '当前模型不支持图片，请更换模型后发送' : '发送'}
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
