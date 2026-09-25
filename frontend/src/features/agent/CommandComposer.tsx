import { loadProjectComposer, restoreDraftMentions } from '../../stores/composerStore';
import { HistoryBanner, RestoredInputResources } from './ProjectHistoryControls';
import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { Paperclip, Send, Sparkles, StopCircle, X } from 'lucide-react';
import { attachmentsApi } from '../../api/attachments';
import { ImagePreview } from '../../components/ui/ImagePreview';
import { IconButton } from '../../components/ui/primitives';
import { llmApi } from '../../api/llm';
import { polishCommand } from '../../stores/textCommandStore';
import { showGlobalError, showGlobalSuccess, showGlobalWarning } from '../../stores/toastStore';
import { skillsApi } from '../../api/skills';
import type { CreateRunRequest, CreateRunScopeInput, LLMProfile, Skill } from '../../api/types';
import { cn } from '../../lib/utils';
import { MAX_MESSAGE_ATTACHMENTS, type ComposerAttachment, useComposerStore } from '../../stores/composerStore';
import { useDeckStore } from '../../stores/deckStore';
import { useProjectStore } from '../../stores/projectStore';
import { orderedSlides } from '../deck/selectors';
import { useRunStore } from '../../stores/runStore';
import { useThreadStore } from '../../stores/threadStore';
import { useShortcutStore } from '../../stores/shortcutStore';
import { matchesShortcut } from '../../lib/shortcuts';
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
import { nextInputShortcutIndex } from './nextInputSuggestions';
import { DOMSelectionReference } from './DOMSelectionReference';

function composerScopeInput(
  composer: ReturnType<typeof useComposerStore.getState>,
  currentSlideId: string | undefined,
): CreateRunScopeInput | null {

  if (composer.scopeSelection === 'current_page') {
    return currentSlideId
      ? { selection: { kind: 'current_page', current_slide_id: currentSlideId } }
      : { selection: { kind: 'all_pages' } };
  }
  if (composer.scopeSelection === 'custom_pages') {
    return composer.customSlideIds.length > 0
      ? { selection: { kind: 'custom_pages', slide_ids: composer.customSlideIds } }
      : null;
  }
  if (composer.scopeSelection === 'custom_sections') {
    return composer.customSectionIds.length > 0
      ? { selection: { kind: 'custom_sections', section_ids: composer.customSectionIds } }
      : null;
  }
  return { selection: { kind: 'all_pages' } };
}

function formatFileSize(size: number): string {
	if (size >= 1024 * 1024) return `${(size / (1024 * 1024)).toFixed(1)} MB`;
	if (size >= 1024) return `${Math.max(1, Math.round(size / 1024))} KB`;
	return `${size} B`;
}

function supportedImageFile(file: File): boolean {
	return ['image/png', 'image/jpeg', 'image/webp'].includes(file.type);
}

export const CommandComposer: React.FC<{ polishToolbarContainer?: HTMLDivElement | null }> = ({ polishToolbarContainer }) => {
  const [menuContainer, setMenuContainer] = useState<HTMLDivElement | null>(null);
  const [text, setText] = useState('');
	const [uploadingCount, setUploadingCount] = useState(0);
  const [isComposing, setIsComposing] = useState(false);
  const [profiles, setProfiles] = useState<LLMProfile[]>([]);
  const [profilesLoading, setProfilesLoading] = useState(true);
  const [profilesError, setProfilesError] = useState('');
  const [skills, setSkills] = useState<Skill[]>([]);
  const [skillsLoading, setSkillsLoading] = useState(true);
  const [suggestionsDismissed, setSuggestionsDismissed] = useState(false);
  const [composerFocused, setComposerFocused] = useState(false);
  const [composerMenuOpen, setComposerMenuOpen] = useState(false);
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
  const planApproved = activeSession.plan?.status === 'active' || activeSession.plan?.status === 'completed';
  const initialRunMode = activeSession.originalRequest?.mode;
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
		? `${activeProjectId ?? ''}:${activeThreadId ?? ''}:${nextInputSuggestions.runId}:${nextInputSuggestions.messageId}`
		: null;
	const projectContentReady = Boolean(
		activeProjectId
		&& contentByProjectId[activeProjectId]
		&& !contentLoadingByProjectId[activeProjectId]
		&& !contentErrorByProjectId[activeProjectId],
	);
	const suggestionsVisible = Boolean(
		suggestionKey
		&& nextInputSuggestions?.status === 'eligible'
		&& nextInputSuggestions.items.length > 0
		&& nextInputSuggestions.items.length <= 3
		&& !runActive
		&& activeThreadId
		&& activeProjectId
		&& activeSession.projectId === activeProjectId
		&& draftPristine
		&& !isComposing
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
	const requiresVision = hasAttachments || (composer.mode === 'execute');

  const activeSnapshot = activeProjectId ? contentByProjectId[activeProjectId] : undefined;
  const slides = useMemo(() => orderedSlides(activeSnapshot), [activeSnapshot]);
  const pageCandidates = useMemo(() => slides.map((slide, index) => ({
    slideId: slide.id,
    ordinal: index + 1,
    title: slide.title,
    keyMessage: slide.spec?.key_message ?? '',
    specState: slide.spec ? 'ready' as const : 'pending' as const,
    htmlState: slide.html_state ?? 'missing' as const,
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
    { id: 'current_page', label: '当前页', selected: composer.scopeSelection === 'current_page', disabled: isEmptyProject },
    { id: 'all_pages', label: '全部页', selected: composer.scopeSelection === 'all_pages' },
  ], [composer.scopeSelection, isEmptyProject]);

  useEffect(() => {
    if (previousProjectId.current === activeProjectId) return;
    previousProjectId.current = activeProjectId;
    setText('');
    editorRef.current?.setPlainText('');
    loadProjectComposer(activeProjectId);
  }, [activeProjectId, resetForProject]);
  useEffect(() => {
    // Apply after loading the project draft, once per approved run. History and
    // reconnect recovery use the same path; background conversations cannot
    // change the visible composer, nor overwrite a later manual mode choice.
    if (activeProjectId && activeThreadId && activeSession.activeRunId && initialRunMode === 'plan' && activeSession.mode === 'execute' && planApproved) {
      useComposerStore.getState().applyApprovedExecution(activeThreadId, activeSession.activeRunId);
    }
  }, [activeProjectId, activeThreadId, activeSession.activeRunId, activeSession.mode, initialRunMode, planApproved]);
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
		// Show the real caret when suggestions arrive, without taking focus from
		// another control, an open dialog, or a text selection in the workspace.
		if (!suggestionsVisible || document.activeElement !== document.body
			|| document.querySelector('[role="dialog"][data-state="open"], [role="menu"][data-state="open"]')
			|| window.getSelection()?.isCollapsed === false) return;
		editorRef.current?.focusEnd();
	}, [suggestionKey, suggestionsVisible]);
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
    let failureNotified = false;
    const loadModels = () => {
      const id = ++request;
      setProfilesLoading(true); setProfilesError('');
      void llmApi.profiles().then((response) => {
        if (!current || id !== request) return;
        failureNotified = false;
        setProfiles(response.profiles);
        useComposerStore.getState().reconcileModels(response.profiles.map((profile) => profile.name), response.default);
      }).catch(() => {
        if (!current || id !== request) return;
        const message = '模型列表加载失败，请刷新后重试';
        setProfilesError(message);
        if (!failureNotified) showGlobalError(message);
        failureNotified = true;
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
    void skillsApi.list()
      .then((response) => {
        if (!current) return;
        const enabledSkills = response.skills.filter((skill) => !skill.disabled && skill.content_state === 'ready');
        setSkills(enabledSkills);
        useComposerStore.getState().reconcileSkills(enabledSkills.map((skill) => skill.id));
      })
      .catch(() => {
        if (!current) return;
        setSkills([]);
        showGlobalWarning('技能列表加载失败');
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
			showGlobalError('仅支持 PNG、JPG 和 WebP 图片');
			return;
		}
		const oversized = files.find((file) => file.size > 10 * 1024 * 1024);
		if (oversized) {
			showGlobalError('图片超过 10 MiB，请压缩后重试');
			return;
		}
		if (activeAttachments.length + uploadingCount + files.length > MAX_MESSAGE_ATTACHMENTS) {
			showGlobalError('每条消息最多添加 8 张图片');
			return;
		}
		let threadId: string;
		try {
			threadId = activeThreadId ?? await ensureActiveThread(activeProjectId);
		} catch (error) {
			showGlobalError(error instanceof Error ? error.message : '创建会话失败，请重试');
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
		if (failure) showGlobalError(failure);
	};

  const submit = async () => {
    const editor = editorRef.current;
    const raw = (editor?.getSubmitText() ?? text).trim();
    const componentNames = editor?.getComponentNames() ?? [];
    const mentionedSlideIds = editor?.getMentionedSlideIds() ?? [];
    const currentSlides = activeProjectId ? orderedSlides(contentByProjectId[activeProjectId]) : [];
    if (mentionedSlideIds.some((id) => !currentSlides.some((slide) => slide.id === id)) || activeDOMSelections.some((selection) => !currentSlides.some((slide) => slide.id === selection.slide_id))) {
      showGlobalError('引用的页面已删除，请移除失效引用后重新发送'); return;
    }
	const apiDOMSelections = activeDOMSelections.map(({ dedupe_key: _dedupeKey, ...selection }) => selection);
	const renameCommand = `${useShortcutStore.getState().bindings['trigger.command'].trigger}rename`;
	if (raw.toLowerCase() === renameCommand || raw.toLowerCase().startsWith(`${renameCommand} `)) {
		if (!activeProjectId) return;
		if (raw.toLocaleLowerCase() !== renameCommand) {
			showGlobalError(`${renameCommand} 不支持参数，请直接使用 ${renameCommand} 立即生成名称`);
			return;
		}
		try {
			const threadId = await ensureActiveThread(activeProjectId);
			void performNamingAction(activeProjectId, threadId, 'generate');
			setText('');
			editorRef.current?.setPlainText('');
		} catch (error) {
			showGlobalError(error instanceof Error ? error.message : '创建会话失败，请重试');
		}
		return;
	}
	if (disabled || commitActive || polishing || briefingActive || !activeProjectId || hasPendingUploads || (!raw && (hasDOMSelections ? !hasDOMIntent : !hasAttachments))) return;
    const projectId = activeProjectId;
    let threadId: string;
    try {
      threadId = await ensureActiveThread(projectId);
    } catch (error) {
      showGlobalError(error instanceof Error ? error.message : '创建会话失败，请重试');
      return;
    }
    if (steering && activeRunId) {
		const accepted = await steerRun(threadId, activeRunId, raw, newClientIdentity('msg'), activeAttachments.map((attachment) => attachment.attachmentId), apiDOMSelections, activeReferences.map((item) => ({ kind: item.kind, ref_id: item.kind === 'image' ? item.attachment.attachmentId : item.selection.selection_id })));
      if (accepted) {
        setText('');
        editorRef.current?.setPlainText('');
		composer.clearThreadDraft(threadId);
      }
      else showGlobalError('追加要求未能加入当前任务；文本已保留，可在任务结束后作为新请求发送');
      return;
    }
    if (profilesError || profilesLoading || !composer.modelProfileName) {
      showGlobalError(profilesError || '模型列表仍在加载，请稍候');
      return;
    }
    const restored = composer.restoredInputs[threadId];
    const scope = restored?.scope ?? composerScopeInput(composer, currentSlide?.id);
    if (!scope) {
      showGlobalError(composer.scopeSelection === 'custom_pages' ? '请至少选择一页' : '请至少选择一章');
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
    const requiresVision = hasAttachments || (request.mode === 'execute');
    if (!selectedProfile) {
      showGlobalError('所选模型已不可用，请重新选择');
      return;
    }
    if (requiresVision && !selectedProfile.capabilities.vision) {
      showGlobalError('当前任务需要页面图片观察，请选择支持页面观察的模型');
      return;
    }

    try {
      if (runStatus === 'paused' && activeRunId) {
        const ended = await cancelRun(threadId, activeRunId, 'superseded');
        if (!ended) {
          showGlobalError('此前任务未能结束，暂时无法发送新消息');
          return;
        }
      }
      const result = await createRun(threadId, request, projectId);
      if (result === 'created') {
        setText('');
        editorRef.current?.setPlainText('');
		composer.clearThreadDraft(threadId);
      }
      else if (result === 'failed') showGlobalError('运行创建失败，请检查时间线中的错误后重试');
    } catch (error) {
      showGlobalError(error instanceof Error ? error.message : '创建会话失败，请重试');
    }
  };

  const cancelActiveRun = async () => {
    if (!activeThreadId || !activeRunId || runStatus === 'canceling') return;
    await cancelRun(activeThreadId, activeRunId);
  };

  const polishText = async () => {
    const instruction = editorRef.current?.getPlainText().trim() ?? '';
    if (disabled || commitActive || polishing || briefingActive || !activeProjectId || !instruction) return;
    const restored = activeThreadId ? composer.restoredInputs[activeThreadId] : undefined;
    const scope = restored?.scope ?? composerScopeInput(composer, currentSlide?.id);
    if (!scope) {
      showGlobalError('请先选择要处理的页面或章节');
      return;
    }
    try {
      const threadId = await ensureActiveThread(activeProjectId);
      await polishCommand(activeProjectId, threadId, {
        instruction, thread_id: threadId, scope, mode: composer.mode,
      });
    } catch (error) {
      showGlobalError(error instanceof Error ? error.message : '润色失败，请重试');
    }
  };

  const executeSlashCommand = async (command: SlashCommandId) => {
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
			showGlobalError(error instanceof Error ? error.message : '创建会话失败，请重试');
		}
		return;
	}
    if (!activeProjectId) return;
    let threadId: string;
    try {
      threadId = await ensureActiveThread(activeProjectId);
    } catch (error) {
      showGlobalError(error instanceof Error ? error.message : '创建会话失败，请重试');
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
    if (id === 'current_page' || id === 'all_pages') composer.setScopeSelection(id);
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
				isComposing: event.isComposing || isComposing || event.repeat,
				code: event.code,
				blocked: blocked || !useShortcutStore.getState().ready,
				targetEditable: Boolean(editable),
				targetIsComposer: Boolean(target?.closest('.composer-prompt-editor')),
			}, useShortcutStore.getState().bindings);
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
    if (useShortcutStore.getState().ready && matchesShortcut(event.nativeEvent, useShortcutStore.getState().bindings['composer.submit'])) {
      event.preventDefault();
      if (!event.repeat) void submit();
    }
  };
	const selectedProfile = profiles.find((profile) => profile.name === composer.modelProfileName);
	const attachmentModelSupported = !hasAttachments || Boolean(selectedProfile?.capabilities.vision);

  return (
    <div className="bg-panel px-3 pb-3 pt-1">
      {polishToolbarContainer && createPortal(
        <IconButton
          label={polishing ? '正在润色表达' : '润色表达'}
          expandableLabel="润色"
          onClick={() => void polishText()}
          disabled={text.trim() === '' || disabled || polishing || commitActive || briefingActive}
        >
          <Sparkles
            className={cn('h-4 w-4', polishing && 'polish-sparkles-active')}
            strokeWidth={1.75}
          />
        </IconButton>,
        polishToolbarContainer,
      )}
      <HistoryBanner />
      <RestoredInputResources />

      <div ref={setMenuContainer} data-steering={steering} className="relative rounded-[18px] border border-border bg-panel-muted shadow-[0_2px_4px_rgba(36,55,84,0.03)] focus-within:border-border-strong">
        <div className="composer-context-bar" role="group" aria-label="模式与范围">
          <ModeSelector
            mode={composer.mode}
            onChange={composer.setIntent}
            disabled={disabled || steering}
          />
          <TargetSelector
            selection={composer.scopeSelection}
            selectedSlideIds={composer.customSlideIds}
            selectedSectionIds={composer.customSectionIds}
            pages={scopePages}
            sections={scopeSections}
            onSelectionChange={composer.setScopeSelection}
            onToggleSlide={composer.toggleCustomSlide}
            onToggleSection={composer.toggleCustomSection}
            disabled={disabled || steering}
            emptyProject={isEmptyProject}
          />
        </div>
        <div className="rounded-[15px_15px_17px_17px] border-t border-border/70 bg-surface">
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
        <div className="relative rounded-t-[15px]">
			{activeReferences.length > 0 && (
				<div className="scrollbar-none flex items-center gap-2 overflow-x-auto px-4 pb-1.5 pt-3" aria-label="当前消息引用">
					{activeReferences.map((reference) => reference.kind === 'image' ? (
						<div key={`${activeProjectId}:${activeThreadId}:${reference.attachment.attachmentId}`} className="relative flex h-[52px] w-44 shrink-0 items-center gap-2 rounded-lg border border-border bg-surface p-1.5 pr-7 hover:bg-accent-soft focus-within:bg-accent-soft">
							{activeProjectId && (
								<ImagePreview
									name={reference.attachment.name}
									thumbnailSrc={attachmentsApi.contentUrl(activeProjectId, reference.attachment.attachmentId)}
									src={attachmentsApi.contentUrl(activeProjectId, reference.attachment.attachmentId, 'original')}
								/>
							)}
							<span className="min-w-0 flex-1">
								<span className="block truncate text-[11px] font-semibold leading-4 text-text-900">{reference.attachment.name}</span>
								<span className="mt-0.5 block text-[11px] leading-3 text-text-600">{formatFileSize(reference.attachment.size)}</span>
							</span>
							<button
								type="button"
								onClick={() => activeThreadId && composer.removeThreadAttachment(activeThreadId, reference.attachment.attachmentId)}
								className="absolute right-1 top-1 inline-flex h-5 w-5 items-center justify-center rounded-md text-text-600 hover:bg-transparent hover:text-danger focus-visible:bg-transparent focus-visible:text-danger"
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
          <PromptComposerEditor
            ref={editorRef}
            menuContainer={menuContainer}
            value={text}
            onChange={setComposerText}
            onKeyDown={handleKeyDown}
            onCompositionChange={setIsComposing}
            placeholder={suggestionsVisible ? '' : composerPlaceholder}
            disabled={disabled}
            readOnly={polishing}
            placeholderContent={suggestionsVisible && nextInputSuggestions
              ? <NextInputSuggestionsPanel items={nextInputSuggestions.items} onSelect={selectSuggestion} />
              : undefined}
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
          </div>
          <div data-composer-control-group="end" className="flex min-w-0 shrink-0 items-center gap-0.5">
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
    </div>
  );
};
