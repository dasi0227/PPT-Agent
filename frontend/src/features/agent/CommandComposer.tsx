import { loadProjectComposer, restoreDraftMentions } from '../../stores/composerStore';
import { HistoryBanner, RestoredInputResources } from './ProjectHistoryControls';
import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { FileImage, Paperclip, Send, Sparkles, StopCircle, X } from 'lucide-react';
import { attachmentsApi } from '../../api/attachments';
import { llmApi } from '../../api/llm';
import { polishCommand } from '../../stores/textCommandStore';
import { showGlobalSuccess } from '../../stores/toastStore';
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
import { ModeSelector } from './ModeSelector';
import { MODE_META } from './modeMeta';
import { ModelSelector } from './ModelSelector';
import { SkillSelector } from './SkillSelector';
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
import { NextInputSuggestionsPanel } from './NextInputSuggestionsPanel';
import { projectHistoryApi } from '../../api/projectHistory';
import { useProjectHistoryStore } from '../../stores/projectHistoryStore';
import { nextInputShortcutIndex } from './nextInputSuggestions';
import { DOMSelectionReference } from './DOMSelectionReference';

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

function formatFileSize(size: number): string {
	if (size >= 1024 * 1024) return `${(size / (1024 * 1024)).toFixed(1)} MB`;
	if (size >= 1024) return `${Math.max(1, Math.round(size / 1024))} KB`;
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
  const [suggestionsDismissed, setSuggestionsDismissed] = useState(false);
  const [composerFocused, setComposerFocused] = useState(false);
  const [composerMenuOpen, setComposerMenuOpen] = useState(false);
  const [verifiedSuggestionKey, setVerifiedSuggestionKey] = useState<string | null>(null);
  const editorRef = useRef<PromptComposerEditorHandle>(null);
	const fileInputRef = useRef<HTMLInputElement>(null);
  const { activeProjectId, contentByProjectId, contentLoadingByProjectId, contentErrorByProjectId } = useProjectStore();
  const { currentSlideId } = useDeckStore();
  const { activeThreadIdByProjectId, ensureActiveThread, performNamingAction } = useThreadStore();
  const { cancelRun, createRun, steerRun } = useRunStore();
  const activeSession = useActiveSession();
  const { status: runStatus, activeRunId, nextInputSuggestions } = activeSession;
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
  const applyContextDefault = composer.applyContextDefault;
  const resetForProject = composer.resetForProject;
  const previousProjectId = useRef<string | null | undefined>(undefined);
  const steering = runStatus === 'running' && Boolean(activeRunId);
  const disabled = !activeProjectId || runStatus === 'creating' || runStatus === 'waiting' || runStatus === 'recovering' || runStatus === 'canceling';
  const runActive = runStatus === 'creating' || runStatus === 'running' || runStatus === 'waiting' || runStatus === 'paused' || runStatus === 'recovering' || runStatus === 'canceling';
  const activeThreadId = activeProjectId ? activeThreadIdByProjectId[activeProjectId] : undefined;
  const activeThreadDraft = activeThreadId ? composer.threadDrafts[activeThreadId] : undefined;
	const activeReferences = activeThreadId ? composer.threadReferences[activeThreadId] ?? [] : [];
	const activeAttachments = activeReferences.flatMap((item) => item.kind === 'image' ? [item.attachment] : []);
	const activeDOMSelections = activeReferences.flatMap((item) => item.kind === 'dom' ? [item.selection] : []);
	const hasAttachments = activeAttachments.length > 0;
	const hasDOMSelections = activeDOMSelections.length > 0;
	const hasDOMIntent = activeDOMSelections.some((selection) => selection.comment.trim() !== '');
	const hasSendableContent = text.trim() !== '' || (hasDOMSelections ? hasDOMIntent : hasAttachments);
	const hasPendingUploads = uploadingCount > 0;
	const activeResourceMentions = activeThreadId ? composer.threadResourceMentions[activeThreadId] : undefined;
	const hasRestoredInput = Boolean(activeThreadId && composer.restoredInputs[activeThreadId]);
	const draftPristine = text.length === 0
		&& activeReferences.length === 0
		&& !hasPendingUploads
		&& !hasRestoredInput
		&& (activeResourceMentions?.component_names?.length ?? 0) === 0
		&& (activeResourceMentions?.mentioned_slide_ids?.length ?? 0) === 0;
	const suggestionKey = nextInputSuggestions
		? `${activeProjectId ?? ''}:${activeThreadId ?? ''}:${nextInputSuggestions.runId}:${nextInputSuggestions.messageId}:${nextInputSuggestions.projectHistoryRevision}`
		: null;
	const historyState = useProjectHistoryStore((state) => activeProjectId ? state.states[activeProjectId] : undefined);
	const historyStateFailed = useProjectHistoryStore((state) => activeProjectId ? state.stateErrorByProjectId[activeProjectId] === true : true);
	const projectContentReady = Boolean(
		activeProjectId
		&& contentByProjectId[activeProjectId]
		&& !contentLoadingByProjectId[activeProjectId]
		&& !contentErrorByProjectId[activeProjectId],
	);
	const suggestionsVisible = Boolean(
		suggestionKey
		&& verifiedSuggestionKey === suggestionKey
		&& nextInputSuggestions?.status === 'eligible'
		&& nextInputSuggestions.items.length > 0
		&& nextInputSuggestions.items.length <= 3
		&& runStatus === 'done'
		&& activeProjectId
		&& activeSession.projectId === activeProjectId
		&& historyState?.revision === nextInputSuggestions.projectHistoryRevision
		&& !historyStateFailed
		&& projectContentReady
		&& draftPristine
		&& !disabled
		&& !polishing
		&& !commitActive
		&& !briefingActive
		&& !suggestionsDismissed
	);
  const setComposerText = useCallback((nextText: string) => {
    setText(nextText);
    if (activeThreadId) {
      useComposerStore.setState((state) => ({ threadResourceMentions: { ...state.threadResourceMentions, [activeThreadId]: {
        component_names: editorRef.current?.getComponentNames() ?? [],
        mentioned_slide_ids: editorRef.current?.getMentionedSlideIds() ?? [],
      } } }));
      composer.setThreadDraft(activeThreadId, nextText);
    }
  }, [activeThreadId, composer]);
  const showCancelButton = Boolean(activeRunId)
    && (runStatus === 'creating' || runStatus === 'running' || runStatus === 'waiting' || runStatus === 'recovering' || runStatus === 'canceling')
	&& activeReferences.length === 0 && text.trim() === '';
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
  const isEmptyProject = Boolean(activeProjectId) && projectContentReady && slides.length === 0;
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
    { id: 'object:spec', label: '设计稿', selected: composer.scopeObject === 'spec', disabled: isEmptyProject },
    { id: 'object:html', label: '幻灯片', selected: composer.scopeObject === 'html', disabled: isEmptyProject },
    { id: 'object:presentation', label: '演示文稿', selected: composer.scopeObject === 'presentation', disabled: isEmptyProject },
    { id: 'object:global', label: '全局资源', selected: composer.scopeObject === 'global' },
  ], [composer.scopeObject, isEmptyProject]);

  useEffect(() => {
    if (previousProjectId.current === activeProjectId) return;
    previousProjectId.current = activeProjectId;
    setText('');
    editorRef.current?.setPlainText('');
    loadProjectComposer(activeProjectId);
  }, [activeProjectId, resetForProject]);
  const loadedDraftThread = useRef<string | null | undefined>(null);
  useEffect(() => {
    const draft = activeThreadDraft ?? '';
    setText(draft);
    if (loadedDraftThread.current !== activeThreadId || editorRef.current?.getPlainText() !== draft) {
      if (activeThreadId) restoreDraftMentions(activeThreadId);
      editorRef.current?.setPlainText(draft);
    }
    loadedDraftThread.current = activeThreadId;
  }, [activeThreadId, activeThreadDraft]);
	useEffect(() => {
		setSuggestionsDismissed(false);
	}, [suggestionKey]);
	useEffect(() => {
		setVerifiedSuggestionKey(null);
		if (!suggestionKey || !activeProjectId || nextInputSuggestions?.status !== 'eligible' || historyStateFailed) return;
		let canceled = false;
		Promise.all([
			projectHistoryApi.state(activeProjectId),
			useProjectStore.getState().loadProjectContent(activeProjectId),
		]).then(([state]) => {
			if (canceled) return;
			useProjectHistoryStore.setState((current) => ({
				states: { ...current.states, [activeProjectId]: state },
				stateErrorByProjectId: { ...current.stateErrorByProjectId, [activeProjectId]: false },
			}));
			const project = useProjectStore.getState();
			if (state.revision === nextInputSuggestions.projectHistoryRevision
				&& project.contentByProjectId[activeProjectId]
				&& !project.contentLoadingByProjectId[activeProjectId]
				&& !project.contentErrorByProjectId[activeProjectId]) {
				setVerifiedSuggestionKey(suggestionKey);
			}
		}).catch(() => {
			if (canceled) return;
			setVerifiedSuggestionKey(null);
			useProjectHistoryStore.setState((current) => ({
				stateErrorByProjectId: { ...current.stateErrorByProjectId, [activeProjectId]: true },
			}));
		});
		return () => { canceled = true; };
	}, [activeProjectId, historyStateFailed, nextInputSuggestions, suggestionKey]);
  useEffect(() => {
    if (!projectContentReady) return;
    applyContextDefault(slides.length > 0);
  }, [activeProjectId, applyContextDefault, projectContentReady, slides.length]);
  useEffect(() => {
    if (activeThreadId && useComposerStore.getState().restoredInputs[activeThreadId]) return;
    useComposerStore.getState().reconcileScopeIds(scopePages.map((page) => page.id), scopeSections.map((section) => section.id));
  }, [activeThreadId, scopePages, scopeSections]);
  useEffect(() => {
    let current = true;
    let request = 0;
    const loadModels = () => {
      const id = ++request;
      setProfilesLoading(true); setProfilesError('');
      void llmApi.profiles().then((response) => {
        if (!current || id !== request) return;
        setProfiles(response.profiles);
        useComposerStore.getState().reconcileModels(response.profiles.map((profile) => profile.name), response.default);
      }).catch(() => {
        if (!current || id !== request) return;
        setProfilesError('模型列表加载失败，请刷新后重试');
      }).finally(() => { if (current && id === request) setProfilesLoading(false); });
    };
    loadModels();
    window.addEventListener('focus', loadModels);
    window.addEventListener('model-settings-saved', loadModels);
    return () => { current = false; window.removeEventListener('focus', loadModels); window.removeEventListener('model-settings-saved', loadModels); };
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
	const apiDOMSelections = activeDOMSelections.map(({ dedupe_key: _dedupeKey, ...selection }) => selection);
	if (/^\/rename(?:\s|$)/i.test(raw)) {
		if (!activeProjectId) return;
		if (raw.toLocaleLowerCase() !== '/rename') {
			setSubmitError('/rename 不支持参数，请直接使用 /rename 立即生成名称');
			return;
		}
		try {
			const threadId = await ensureActiveThread(activeProjectId);
			void performNamingAction(activeProjectId, threadId, 'generate');
			setText('');
			editorRef.current?.setPlainText('');
		} catch (error) {
			setSubmitError(error instanceof Error ? error.message : '创建会话失败，请重试');
		}
		return;
	}
	if (disabled || commitActive || polishing || briefingActive || !activeProjectId || hasPendingUploads || (!raw && (hasDOMSelections ? !hasDOMIntent : !hasAttachments))) return;
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
		const accepted = await steerRun(threadId, activeRunId, raw, newClientIdentity('msg'), activeAttachments.map((attachment) => attachment.attachmentId), apiDOMSelections, activeReferences.map((item) => ({ kind: item.kind, ref_id: item.kind === 'image' ? item.attachment.attachmentId : item.selection.selection_id })));
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
    const restored = composer.restoredInputs[threadId];
    const scope = restored?.scope ?? composerScopeInput(composer, currentSlide?.id);
    if (!scope) {
      setSubmitError(composer.scopeSelection === 'custom_pages' ? '请至少选择一页' : '请至少选择一章');
      return;
    }
    const request: CreateRunRequest = {
      ...restored,
      ...(restored ? { restored_checkpoint: true } : {}),
      client_request_id: newClientIdentity('req'),
      model: composer.modelSelectionExplicit ? composer.modelProfileName ?? undefined : undefined,
      scope,
      mode: composer.mode,
      instruction: raw,
		...(hasAttachments ? { attachment_ids: activeAttachments.map((attachment) => attachment.attachmentId) } : {}),
		...(hasDOMSelections ? { dom_selections: apiDOMSelections } : {}),
		...(activeReferences.length > 0 ? { reference_order: activeReferences.map((item) => ({ kind: item.kind, ref_id: item.kind === 'image' ? item.attachment.attachmentId : item.selection.selection_id })) } : {}),
      ...(composer.selectedSkillIds.length > 0 ? { skill_ids: composer.selectedSkillIds } : {}),
      ...((componentNames.length > 0 || restored?.component_names?.length) ? { component_names: [...new Set([...(restored?.component_names ?? []), ...componentNames])] } : {}),
      ...((mentionedSlideIds.length > 0 || restored?.mentioned_slide_ids?.length) ? { mentioned_slide_ids: [...new Set([...(restored?.mentioned_slide_ids ?? []), ...mentionedSlideIds])] } : {}),
    };
    // Default routing omits request.model; validate the effective composer selection.
    const selectedProfile = profiles.find((profile) => profile.name === composer.modelProfileName);
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

  const polishText = async () => {
    const instruction = editorRef.current?.getPlainText().trim() ?? '';
    if (disabled || commitActive || polishing || briefingActive || !activeProjectId || !instruction) return;
    const restored = activeThreadId ? composer.restoredInputs[activeThreadId] : undefined;
    const scope = restored?.scope ?? composerScopeInput(composer, currentSlide?.id);
    if (!scope) {
      setSubmitError('请先选择要处理的页面或章节');
      return;
    }
    setSubmitError('');
    try {
      const threadId = await ensureActiveThread(activeProjectId);
      await polishCommand(activeProjectId, threadId, {
        instruction, thread_id: threadId, scope, mode: composer.mode,
      });
    } catch (error) {
      setSubmitError(error instanceof Error ? error.message : '润色失败，请重试');
    }
  };

  const executeSlashCommand = async (command: SlashCommandId) => {
    setSubmitError('');
    if (command === 'execute' || command === 'plan' || command === 'grill' || command === 'chat') {
      composer.setIntent(command);
      showGlobalSuccess(`成功切换到${MODE_META[command].label}模式`);
      return;
    }
    if (command === 'polish') {
      await polishText();
      return;
    }
	if (command === 'rename') {
		if (!activeProjectId) return;
		try {
			const threadId = await ensureActiveThread(activeProjectId);
			void performNamingAction(activeProjectId, threadId, 'generate');
		} catch (error) {
			setSubmitError(error instanceof Error ? error.message : '创建会话失败，请重试');
		}
		return;
	}
    if (!activeProjectId) return;
    let threadId: string;
    try {
      threadId = await ensureActiveThread(activeProjectId);
    } catch (error) {
      setSubmitError(error instanceof Error ? error.message : '创建会话失败，请重试');
      return;
    }
    if (command === 'commit') {
      await startCommit(activeProjectId, threadId);
      return;
    }
    if (command === 'kickoff' || command === 'handoff') {
      await generateBriefing(activeProjectId, threadId, command);
    }
  };

  const selectTargetOption = (id: string) => {
    const [kind, object] = id.split(':');
    if (kind !== 'object' || !['spec', 'html', 'presentation', 'global'].includes(object)) return;
    composer.setScopeObject(object as 'spec' | 'html' | 'presentation' | 'global');
  };

	const selectSuggestion = useCallback((value: string) => {
		setComposerText(value);
		requestAnimationFrame(() => {
			editorRef.current?.setPlainText(value);
			editorRef.current?.focusEnd();
		});
	}, [setComposerText]);

	useEffect(() => {
		if (!suggestionsVisible) return;
		const onKeyDown = (event: globalThis.KeyboardEvent) => {
			const target = event.target instanceof HTMLElement ? event.target : null;
			const editable = target?.closest('input, textarea, select, [contenteditable="true"]');
			const blocked = composerMenuOpen || Boolean(document.querySelector(
				'[role="dialog"][data-state="open"], [role="menu"][data-state="open"], [role="listbox"], [role="grid"]',
			));
			const index = nextInputShortcutIndex({
				altKey: event.altKey,
				metaKey: event.metaKey,
				ctrlKey: event.ctrlKey,
				shiftKey: event.shiftKey,
				isComposing: event.isComposing || isComposing,
				code: event.code,
				blocked,
				targetEditable: Boolean(editable),
				targetIsComposer: Boolean(target?.closest('.composer-prompt-editor')),
			});
			if (index === null || !nextInputSuggestions?.items[index]) return;
			event.preventDefault();
			selectSuggestion(nextInputSuggestions.items[index]);
		};
		window.addEventListener('keydown', onKeyDown);
		return () => window.removeEventListener('keydown', onKeyDown);
	}, [composerMenuOpen, isComposing, nextInputSuggestions, selectSuggestion, suggestionsVisible]);

  const handleKeyDown = (event: React.KeyboardEvent) => {
    if (event.nativeEvent.isComposing || isComposing || polishing || commitActive || briefingActive) return;
		if (event.key === 'Escape' && suggestionsVisible && composerFocused) {
			event.preventDefault();
			setSuggestionsDismissed(true);
			return;
		}
    if (event.key === 'Enter' && (isMac() ? event.metaKey : event.ctrlKey)) {
      event.preventDefault();
      void submit();
    }
  };
	const selectedProfile = profiles.find((profile) => profile.name === composer.modelProfileName);
	const attachmentModelSupported = !hasAttachments || Boolean(selectedProfile?.capabilities.vision);

  return (
    <div className="bg-panel px-3 pb-3 pt-1">
      <HistoryBanner />
      <RestoredInputResources />
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
			{activeReferences.length > 0 && (
				<div className="flex gap-2 overflow-x-auto px-3 pb-1.5 pt-3" aria-label="当前消息引用">
					{activeReferences.map((reference) => reference.kind === 'image' ? (
						<div key={reference.attachment.attachmentId} className="relative grid w-44 shrink-0 grid-cols-[38px_minmax(0,1fr)] items-center gap-2 rounded-lg border border-border bg-surface p-1.5 pr-7 shadow-sm">
							<span className="grid h-[38px] w-[38px] place-items-center rounded-md bg-panel-muted text-text-600" aria-hidden="true">
								<FileImage className="h-5 w-5" strokeWidth={1.75} />
							</span>
							<span className="min-w-0">
								<span className="block truncate text-[11px] font-semibold leading-4 text-text-900">{reference.attachment.name}</span>
								<span className="mt-0.5 block text-[11px] leading-3 text-text-600">{formatFileSize(reference.attachment.size)}</span>
							</span>
							<button
								type="button"
								onClick={() => activeThreadId && composer.removeThreadAttachment(activeThreadId, reference.attachment.attachmentId)}
								className="absolute right-1 top-1 inline-flex h-5 w-5 items-center justify-center rounded-md text-text-600 hover:bg-panel-muted hover:text-text-900"
								aria-label={`移除 ${reference.attachment.name}`}
								title="从当前消息移除"
							>
								<X className="h-3.5 w-3.5" strokeWidth={1.75} />
							</button>
						</div>
					) : (
						<DOMSelectionReference
							key={reference.selection.selection_id}
							selection={reference.selection}
							editing={composer.editingSelectionIdByThread[activeThreadId ?? ''] === reference.selection.selection_id}
							onEditingChange={(editing) => { if (activeThreadId) composer.setEditingDOMSelection(activeThreadId, editing ? reference.selection.selection_id : undefined); }}
							onCommentChange={(comment) => { if (activeThreadId) composer.updateThreadDOMSelection(activeThreadId, reference.selection.selection_id, { comment }); }}
							onNavigate={() => {
								if (!activeThreadId) return;
								if (reference.selection.status !== 'page_deleted') {
									useDeckStore.getState().setCurrentSlideId(reference.selection.slide_id);
									useDeckStore.getState().exitOverview();
								}
							}}
							onRemove={() => { if (activeThreadId) composer.removeThreadDOMSelection(activeThreadId, reference.selection.selection_id); }}
							onFocusComposer={() => editorRef.current?.focusEnd()}
						/>
					))}
				</div>
          )}
		  {suggestionsVisible && nextInputSuggestions && (
			<NextInputSuggestionsPanel items={nextInputSuggestions.items} onSelect={selectSuggestion} />
		  )}
          <PromptComposerEditor
            ref={editorRef}
            value={text}
            onChange={setComposerText}
            onKeyDown={handleKeyDown}
            onCompositionChange={setIsComposing}
            placeholder={suggestionsVisible ? '' : composerPlaceholder}
            disabled={disabled}
            readOnly={polishing}
			suggestionsVisible={suggestionsVisible}
			onFocusChange={(focused) => {
				setComposerFocused(focused);
				if (!focused) setSuggestionsDismissed(false);
			}}
			onMenuOpenChange={setComposerMenuOpen}
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
        <div className="flex min-w-0 items-center justify-between gap-3 px-3 pb-2">
          <div data-composer-control-group="start" className="flex min-w-0 items-center gap-0.5">
			<button
				type="button"
				onClick={() => fileInputRef.current?.click()}
				disabled={disabled}
				className="composer-attach-button inline-flex h-7 min-w-0 shrink-0 items-center gap-1 rounded-md border border-transparent bg-transparent px-2 text-[11px] font-medium text-text-600 transition-colors hover:bg-panel-muted hover:text-text-900 focus-visible:bg-panel-muted focus-visible:text-text-900 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-45"
				aria-label="选择图片"
				title="选择图片"
			>
				<Paperclip className="h-3.5 w-3.5" strokeWidth={1.75} />
				<span className="composer-attach-label shrink-0 whitespace-nowrap">附件</span>
			</button>
            <SkillSelector
              skills={skills}
              selectedIds={composer.selectedSkillIds}
              loading={skillsLoading}
              disabled={disabled || steering}
              onToggle={composer.toggleSkill}
            />
            <ModeSelector
              mode={composer.mode}
              onChange={composer.setIntent}
              disabled={disabled || steering}
            />
          </div>
          <div data-composer-control-group="end" className="flex min-w-0 shrink-0 items-center gap-0.5">
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
              emptyProject={isEmptyProject}
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
				disabled={!hasSendableContent || hasPendingUploads || !attachmentModelSupported || disabled || commitActive || polishing || briefingActive || (!steering && (scopeSelectionEmpty || profilesLoading || Boolean(profilesError)))}
                className="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-accent-soft text-accent disabled:opacity-50"
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
