import {
  forwardRef,
  useCallback,
  useEffect,
  useImperativeHandle,
  useMemo,
  useRef,
  useState,
  type ClipboardEvent,
  type FormEvent,
  type KeyboardEvent,
} from 'react';
import {
  ArrowLeft,
  Blocks,
  Check,
  ChevronRight,
  Cpu,
  Crosshair,
  Footprints,
  GalleryThumbnails,
  GitCommitHorizontal,
  Handshake,
  ListChecks,
  MessageCircleQuestion,
  MessagesSquare,
  NotebookText,
  WandSparkles,
} from 'lucide-react';
import type { ComponentReference, Prompt } from '../../api/types';
import { useComponentStore } from '../../stores/componentStore';
import { usePromptStore } from '../../stores/promptStore';
import {
  findComponentTrigger,
  findCommandTrigger,
  findPageTrigger,
  findPromptTrigger,
  findSummaryTrigger,
  MAX_COMPONENT_MENTIONS,
  MAX_PAGE_MENTIONS,
  matchComponents,
  matchSlashCommands,
  matchPages,
  matchPrompts,
  pageDisplayName,
  navigateCommandMenu,
  type CommandMenuLevel,
  type PageMentionCandidate,
  type PromptTrigger,
  type ResolvedSlashCommand,
  type SlashCommandId,
} from './promptMatching';
import './promptComposer.css';

export interface PromptComposerEditorHandle {
  getPlainText: () => string;
  getSubmitText: () => string;
  getComponentNames: () => string[];
  getMentionedSlideIds: () => string[];
  setPlainText: (value: string) => void;
  focusEnd: () => void;
  captureSelection: () => Range | null;
  restoreSelection: (range: Range | null) => void;
}

type ComposerTrigger = PromptTrigger & { kind: 'prompt' | 'component' | 'page' | 'summary' | 'command' };

export interface SlashMenuOption {
  id: string;
  label: string;
  description?: string;
  selected?: boolean;
  disabled?: boolean;
}

const SUMMARY_COLUMNS = [
  { kind: 'page' as const, label: '页面' },
  { kind: 'component' as const, label: '组件' },
  { kind: 'prompt' as const, label: '提示词' },
];

interface PromptComposerEditorProps {
  value: string;
  onChange: (value: string) => void;
  onKeyDown: (event: KeyboardEvent<HTMLDivElement>) => void;
  onCompositionChange: (composing: boolean) => void;
  placeholder: string;
  disabled: boolean;
  readOnly: boolean;
  pages?: PageMentionCandidate[];
  slashCommands?: ResolvedSlashCommand[];
  modelOptions?: SlashMenuOption[];
  targetOptions?: SlashMenuOption[];
  onSlashCommand?: (command: SlashCommandId) => void;
  onModelOption?: (id: string) => void;
  onTargetOption?: (id: string) => void;
}

function nodePlainText(node: Node): string {
  if (node.nodeType === Node.TEXT_NODE) return node.textContent ?? '';
  if (node instanceof HTMLBRElement) return '\n';
  let result = '';
  node.childNodes.forEach((child) => {
    const block = child instanceof HTMLDivElement || child instanceof HTMLParagraphElement;
    if (block && result && !result.endsWith('\n')) result += '\n';
    result += nodePlainText(child);
  });
  return result;
}

function serializeComposerText(root: HTMLElement): string {
  if (root.childNodes.length === 1 && root.firstChild instanceof HTMLBRElement) return '';
  return nodePlainText(root);
}

function nodeSubmitText(node: Node): string {
  if (node instanceof HTMLElement && node.dataset.slideId) {
    return `${node.textContent ?? ''}⟨${node.dataset.slideId}⟩`;
  }
  if (node.nodeType === Node.TEXT_NODE) return node.textContent ?? '';
  if (node instanceof HTMLBRElement) return '\n';
  let result = '';
  node.childNodes.forEach((child) => {
    const block = child instanceof HTMLDivElement || child instanceof HTMLParagraphElement;
    if (block && result && !result.endsWith('\n')) result += '\n';
    result += nodeSubmitText(child);
  });
  return result;
}

function serializeSubmitText(root: HTMLElement): string {
  if (root.childNodes.length === 1 && root.firstChild instanceof HTMLBRElement) return '';
  return nodeSubmitText(root);
}

function setPlainTextContent(root: HTMLElement, value: string) {
  root.replaceChildren();
  if (value) root.append(document.createTextNode(value));
}

function pointAtOffset(root: HTMLElement, target: number): { node: Node; offset: number } {
  let remaining = Math.max(0, target);
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_ALL, {
    acceptNode(node) {
      if (node.nodeType === Node.TEXT_NODE || node instanceof HTMLBRElement) {
        return NodeFilter.FILTER_ACCEPT;
      }
      return NodeFilter.FILTER_SKIP;
    },
  });
  let node = walker.nextNode();
  while (node) {
    const length = node instanceof HTMLBRElement ? 1 : (node.textContent?.length ?? 0);
    if (!(node instanceof HTMLBRElement) && remaining <= length) {
      return { node, offset: remaining };
    }
    if (node instanceof HTMLBRElement && remaining === 0) {
      const parent = node.parentNode ?? root;
      return { node: parent, offset: Array.from(parent.childNodes).indexOf(node) };
    }
    remaining -= length;
    node = walker.nextNode();
  }
  return { node: root, offset: root.childNodes.length };
}

function caretOffset(root: HTMLElement): number | null {
  const selection = window.getSelection();
  if (!selection?.rangeCount || !selection.isCollapsed || !root.contains(selection.anchorNode)) return null;
  const range = selection.getRangeAt(0).cloneRange();
  range.selectNodeContents(root);
  range.setEnd(selection.anchorNode!, selection.anchorOffset);
  const container = document.createElement('div');
  container.append(range.cloneContents());
  return serializeComposerText(container).length;
}

function placeCaret(root: HTMLElement, offset: number) {
  const point = pointAtOffset(root, offset);
  const range = document.createRange();
  range.setStart(point.node, point.offset);
  range.collapse(true);
  const selection = window.getSelection();
  selection?.removeAllRanges();
  selection?.addRange(range);
}

function insertPlainText(root: HTMLElement, value: string) {
  const selection = window.getSelection();
  if (!selection?.rangeCount || !root.contains(selection.anchorNode)) return;
  const range = selection.getRangeAt(0);
  range.deleteContents();
  const text = document.createTextNode(value);
  range.insertNode(text);
  const next = document.createRange();
  next.setStart(text, value.length);
  next.collapse(true);
  selection.removeAllRanges();
  selection.addRange(next);
}

function pageStatus(page: PageMentionCandidate) {
  const spec = page.specState === 'ready'
    ? { label: '设计稿已就绪', dot: 'bg-success' }
    : { label: '设计稿未生成', dot: 'bg-danger' };
  const html = page.htmlState === 'fresh'
    ? { label: '幻灯片已就绪', dot: 'bg-success' }
    : page.htmlState === 'not_materialized'
      ? { label: '幻灯片未生成', dot: 'bg-danger' }
      : { label: '幻灯片待更新', dot: 'bg-warning' };
  return { spec, html };
}

export const PromptComposerEditor = forwardRef<PromptComposerEditorHandle, PromptComposerEditorProps>(
  function PromptComposerEditor({
    value,
    onChange,
    onKeyDown,
    onCompositionChange,
    placeholder,
    disabled,
    readOnly,
    pages = [],
    slashCommands = [],
    modelOptions = [],
    targetOptions = [],
    onSlashCommand,
    onModelOption,
    onTargetOption,
  }, forwardedRef) {
    const editorRef = useRef<HTMLDivElement>(null);
    const menuRef = useRef<HTMLDivElement>(null);
    const composingRef = useRef(false);
    const dismissedRef = useRef('');
    const [trigger, setTrigger] = useState<ComposerTrigger | null>(null);
    const triggerRef = useRef<ComposerTrigger | null>(null);
    const [activeIndex, setActiveIndex] = useState(0);
    const [summaryCol, setSummaryCol] = useState(0);
    const [summaryRow, setSummaryRow] = useState(0);
    const [commandLevel, setCommandLevel] = useState<CommandMenuLevel>('root');
    const prompts = usePromptStore((state) => state.prompts);
    const version = usePromptStore((state) => state.version);
    const loadPrompts = usePromptStore((state) => state.load);
    const components = useComponentStore((state) => state.components);
    const componentVersion = useComponentStore((state) => state.version);
    const loadComponents = useComponentStore((state) => state.load);
    const pagesById = useMemo(() => new Map(pages.map((page) => [page.slideId, page])), [pages]);
    const pageSignature = pages.map((page) => (
      `${page.slideId}:${page.ordinal}:${page.title}:${page.specState}:${page.htmlState}`
    )).join('|');
    const isSummary = trigger?.kind === 'summary';
    const promptCandidates = trigger && (trigger.kind === 'prompt' || isSummary) ? matchPrompts(prompts, trigger.query) : [];
    const componentCandidates = trigger && (trigger.kind === 'component' || isSummary) ? matchComponents(components, trigger.query) : [];
    const pageCandidates = trigger && (trigger.kind === 'page' || isSummary) ? matchPages(pages, trigger.query) : [];
    const commandCandidates = trigger?.kind === 'command' ? matchSlashCommands(slashCommands, trigger.query) : [];
    const commandOptions = commandLevel === 'model' ? modelOptions : targetOptions;
    // 汇总面板三列（页面 · 组件 · 提示词），列内候选沿用各自匹配规则
    const summaryColumns = [pageCandidates, componentCandidates, promptCandidates];
    const clampSummaryRow = useCallback(
      (length: number) => (length ? Math.max(0, Math.min(summaryRow, length - 1)) : -1),
      [summaryRow],
    );
    const activeSummaryRow = clampSummaryRow(summaryColumns[summaryCol].length);
    const candidateCount = trigger?.kind === 'command'
      ? commandLevel === 'root' ? commandCandidates.length : commandOptions.length
      : trigger?.kind === 'component'
      ? componentCandidates.length
      : trigger?.kind === 'page'
        ? pageCandidates.length
        : promptCandidates.length;
    const commandCandidateDisabled = commandLevel === 'root'
      ? commandCandidates.map((command) => command.disabled)
      : commandOptions.map((option) => Boolean(option.disabled));
    const hasConfiguredCandidates = trigger?.kind === 'command'
      ? commandLevel === 'root' || commandOptions.length > 0
      : trigger?.kind === 'component'
      ? components.some((component) => !component.disabled)
      : trigger?.kind === 'page'
        ? pages.length > 0
        : prompts.some((prompt) => !prompt.disabled);

    const updateTrigger = useCallback(() => {
      const editor = editorRef.current;
      if (!editor || disabled || readOnly || composingRef.current || document.activeElement !== editor) {
        triggerRef.current = null;
        setTrigger((current) => current === null ? current : null);
        return;
      }
      const offset = caretOffset(editor);
      if (offset == null) {
        triggerRef.current = null;
        setTrigger((current) => current === null ? current : null);
        return;
      }
      const text = serializeComposerText(editor);
      const promptTrigger = findPromptTrigger(text, offset);
      const componentTrigger = findComponentTrigger(text, offset);
      const pageTrigger = findPageTrigger(text, offset);
      const summaryTrigger = findSummaryTrigger(text, offset);
      const commandTrigger = findCommandTrigger(text, offset);
      const next: ComposerTrigger | null = commandTrigger
        ? { ...commandTrigger, kind: 'command' }
        : promptTrigger
          ? { ...promptTrigger, kind: 'prompt' }
          : componentTrigger
          ? { ...componentTrigger, kind: 'component' }
          : pageTrigger
            ? { ...pageTrigger, kind: 'page' }
            : summaryTrigger
              ? { ...summaryTrigger, kind: 'summary' }
              : null;
      const signature = next ? `${next.kind}:${next.start}:${next.end}:${next.query}` : '';
      if (!next || dismissedRef.current === signature) {
        triggerRef.current = null;
        setTrigger((current) => current === null ? current : null);
        return;
      }
      const current = triggerRef.current;
      const currentSignature = current
        ? `${current.kind}:${current.start}:${current.end}:${current.query}`
        : '';
      if (currentSignature === signature) return;
      setActiveIndex(0);
      if (next.kind === 'command' && current?.kind !== 'command') {
        setCommandLevel('root');
      }
      // 首次进入汇总面板：高亮第一个非空列的首项
      if (next.kind === 'summary' && current?.kind !== 'summary') {
        const cols = [
          matchPages(pages, next.query).length,
          matchComponents(components, next.query).length,
          matchPrompts(prompts, next.query).length,
        ];
        const firstNonEmpty = cols.findIndex((length) => length > 0);
        setSummaryCol(firstNonEmpty < 0 ? 0 : firstNonEmpty);
        setSummaryRow(0);
      }
      triggerRef.current = next;
      setTrigger(next);
    }, [components, disabled, pages, prompts, readOnly]);

    useEffect(() => {
      triggerRef.current = trigger;
    }, [trigger]);

    useImperativeHandle(forwardedRef, () => ({
      getPlainText: () => editorRef.current ? serializeComposerText(editorRef.current) : '',
      getSubmitText: () => editorRef.current ? serializeSubmitText(editorRef.current) : '',
      getComponentNames: () => {
        const editor = editorRef.current;
        if (!editor) return [];
        const seen = new Set<string>();
        const names: string[] = [];
        editor.querySelectorAll<HTMLElement>('[data-component-name]').forEach((fragment) => {
          const name = fragment.dataset.componentName?.trim() ?? '';
          if (name && !seen.has(name) && names.length < MAX_COMPONENT_MENTIONS) {
            seen.add(name);
            names.push(name);
          }
        });
        return names;
      },
      getMentionedSlideIds: () => {
        const editor = editorRef.current;
        if (!editor) return [];
        const seen = new Set<string>();
        const ids: string[] = [];
        editor.querySelectorAll<HTMLElement>('[data-slide-id]').forEach((fragment) => {
          const id = fragment.dataset.slideId?.trim() ?? '';
          if (id && !seen.has(id) && ids.length < MAX_PAGE_MENTIONS) {
            seen.add(id);
            ids.push(id);
          }
        });
        return ids;
      },
      setPlainText: (nextValue) => {
        const editor = editorRef.current;
        if (!editor) return;
        setPlainTextContent(editor, nextValue);
        editor.dataset.empty = String(nextValue.length === 0);
        setTrigger(null);
      },
      focusEnd: () => {
        const editor = editorRef.current;
        if (!editor || disabled || readOnly) return;
        editor.focus();
        placeCaret(editor, serializeComposerText(editor).length);
      },
      captureSelection: () => {
        const editor = editorRef.current;
        const selection = window.getSelection();
        if (!editor || !selection?.rangeCount || !editor.contains(selection.anchorNode)) return null;
        return selection.getRangeAt(0).cloneRange();
      },
      restoreSelection: (range) => {
        const editor = editorRef.current;
        if (!editor || !range || disabled || readOnly) return;
        editor.focus();
        const selection = window.getSelection();
        selection?.removeAllRanges();
        selection?.addRange(range);
      },
    }), [disabled, readOnly]);

    useEffect(() => {
      const editor = editorRef.current;
      if (!editor || serializeComposerText(editor) === value) return;
      setPlainTextContent(editor, value);
      editor.dataset.empty = String(value.length === 0);
      setTrigger(null);
    }, [value]);

    useEffect(() => {
      const selectionChanged = () => updateTrigger();
      document.addEventListener('selectionchange', selectionChanged);
      return () => document.removeEventListener('selectionchange', selectionChanged);
    }, [updateTrigger]);

    useEffect(() => {
      updateTrigger();
    }, [componentVersion, updateTrigger, version]);

    useEffect(() => {
      const editor = editorRef.current;
      if (!editor) return;
      let changed = false;
      editor.querySelectorAll<HTMLElement>('[data-slide-id]').forEach((fragment) => {
        const page = pagesById.get(fragment.dataset.slideId ?? '');
        fragment.classList.toggle('composer-page-fragment-invalid', !page);
        if (!page) return;
        const displayName = pageDisplayName(page);
        if (fragment.textContent !== displayName) {
          fragment.textContent = displayName;
          changed = true;
        }
      });
      if (changed) onChange(serializeComposerText(editor));
    }, [onChange, pageSignature, pagesById]);

    useEffect(() => {
      if (isSummary) {
        menuRef.current?.querySelector<HTMLElement>(`[data-summary-cell="${summaryCol}:${activeSummaryRow}"]`)
          ?.scrollIntoView({ block: 'nearest' });
        return;
      }
      menuRef.current?.querySelector<HTMLElement>(`[data-candidate-index="${activeIndex}"]`)
        ?.scrollIntoView({ block: 'nearest' });
    }, [activeIndex, activeSummaryRow, isSummary, summaryCol]);

    const syncValue = () => {
      const editor = editorRef.current;
      if (!editor) return;
      const plainText = serializeComposerText(editor);
      editor.dataset.empty = String(plainText.length === 0);
      onChange(plainText);
    };

    const applyPrompt = (prompt: Prompt) => {
      const editor = editorRef.current;
      if (!editor || !trigger) return;
      const start = pointAtOffset(editor, trigger.start);
      const end = pointAtOffset(editor, trigger.end);
      const range = document.createRange();
      range.setStart(start.node, start.offset);
      range.setEnd(end.node, end.offset);
      range.deleteContents();
      const promptFragment = document.createElement('span');
      promptFragment.className = 'composer-prompt-fragment';
      promptFragment.dataset.promptId = prompt.id;
      promptFragment.textContent = prompt.value;
      const space = document.createTextNode(' ');
      range.insertNode(space);
      range.insertNode(promptFragment);
      editor.normalize();
      const selection = window.getSelection();
      const next = document.createRange();
      next.setStartAfter(space);
      next.collapse(true);
      selection?.removeAllRanges();
      selection?.addRange(next);
      dismissedRef.current = '';
      setTrigger(null);
      syncValue();
      editor.focus();
    };

    const applyComponent = (component: ComponentReference) => {
      const editor = editorRef.current;
      if (!editor || !trigger || (trigger.kind !== 'component' && trigger.kind !== 'summary')) return;
      const start = pointAtOffset(editor, trigger.start);
      const end = pointAtOffset(editor, trigger.end);
      const range = document.createRange();
      range.setStart(start.node, start.offset);
      range.setEnd(end.node, end.offset);
      range.deleteContents();
      const componentFragment = document.createElement('span');
      componentFragment.className = 'composer-component-fragment';
      componentFragment.dataset.componentName = component.name;
      componentFragment.textContent = component.name;
      const space = document.createTextNode(' ');
      range.insertNode(space);
      range.insertNode(componentFragment);
      editor.normalize();
      const selection = window.getSelection();
      const next = document.createRange();
      next.setStartAfter(space);
      next.collapse(true);
      selection?.removeAllRanges();
      selection?.addRange(next);
      dismissedRef.current = '';
      setTrigger(null);
      syncValue();
      editor.focus();
    };

    const applyPage = (page: PageMentionCandidate) => {
      const editor = editorRef.current;
      if (!editor || !trigger || (trigger.kind !== 'page' && trigger.kind !== 'summary')) return;
      const start = pointAtOffset(editor, trigger.start);
      const end = pointAtOffset(editor, trigger.end);
      const range = document.createRange();
      range.setStart(start.node, start.offset);
      range.setEnd(end.node, end.offset);
      range.deleteContents();
      const pageFragment = document.createElement('span');
      pageFragment.className = 'composer-page-fragment';
      pageFragment.dataset.slideId = page.slideId;
      pageFragment.contentEditable = 'false';
      pageFragment.textContent = pageDisplayName(page);
      const space = document.createTextNode(' ');
      range.insertNode(space);
      range.insertNode(pageFragment);
      editor.normalize();
      const selection = window.getSelection();
      const next = document.createRange();
      next.setStartAfter(space);
      next.collapse(true);
      selection?.removeAllRanges();
      selection?.addRange(next);
      dismissedRef.current = '';
      setTrigger(null);
      syncValue();
      editor.focus();
    };

    const clearCommandTrigger = () => {
      const editor = editorRef.current;
      if (!editor || !trigger || trigger.kind !== 'command') return false;
      const start = pointAtOffset(editor, trigger.start);
      const end = pointAtOffset(editor, trigger.end);
      const range = document.createRange();
      range.setStart(start.node, start.offset);
      range.setEnd(end.node, end.offset);
      range.deleteContents();
      range.collapse(true);
      editor.normalize();
      const selection = window.getSelection();
      selection?.removeAllRanges();
      selection?.addRange(range);
      dismissedRef.current = '';
      setTrigger(null);
      setCommandLevel('root');
      syncValue();
      editor.focus();
      return true;
    };

    const applyCommand = (command: ResolvedSlashCommand) => {
      if (command.disabled) return;
      if (command.submenu) {
        const options = command.submenu === 'model' ? modelOptions : targetOptions;
        setCommandLevel(command.submenu);
        setActiveIndex(options.findIndex((option) => !option.disabled));
        return;
      }
      if (clearCommandTrigger()) onSlashCommand?.(command.id);
    };

    const applyCommandOption = (option: SlashMenuOption) => {
      if (option.disabled || !clearCommandTrigger()) return;
      if (commandLevel === 'model') onModelOption?.(option.id);
      else onTargetOption?.(option.id);
    };

    // 汇总面板选中：按当前列分派到对应的插入逻辑
    const applySummaryActive = () => {
      const column = summaryColumns[summaryCol];
      const row = clampSummaryRow(column.length);
      if (row < 0) return; // 空列：Enter 无效
      const kind = SUMMARY_COLUMNS[summaryCol].kind;
      if (kind === 'page') applyPage(column[row] as PageMentionCandidate);
      else if (kind === 'component') applyComponent(column[row] as ComponentReference);
      else applyPrompt(column[row] as Prompt);
    };

    const handleInput = (_event: FormEvent<HTMLDivElement>) => {
      dismissedRef.current = '';
      syncValue();
      requestAnimationFrame(updateTrigger);
    };

    const handleEditorKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
      if (trigger && !event.nativeEvent.isComposing && !composingRef.current) {
        if (trigger.kind === 'command' && ['ArrowUp', 'ArrowDown', 'Enter', 'Escape'].includes(event.key)) {
          const result = navigateCommandMenu(
            commandLevel,
            activeIndex,
            event.key as 'ArrowUp' | 'ArrowDown' | 'Enter' | 'Escape',
            commandCandidateDisabled,
          );
          event.preventDefault();
          if (result.action === 'close') {
            dismissedRef.current = `${trigger.kind}:${trigger.start}:${trigger.end}:${trigger.query}`;
            setTrigger(null);
            return;
          }
          if (result.level !== commandLevel) {
            setCommandLevel(result.level);
            setActiveIndex(result.activeIndex);
            return;
          }
          if (result.action === 'select') {
            if (commandLevel === 'root') {
              const command = commandCandidates[activeIndex];
              if (command) applyCommand(command);
            } else {
              const option = commandOptions[activeIndex];
              if (option) applyCommandOption(option);
            }
            return;
          }
          setActiveIndex(result.activeIndex);
          return;
        }
        if (trigger.kind === 'summary') {
          if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') {
            event.preventDefault();
            const direction = event.key === 'ArrowRight' ? 1 : -1;
            setSummaryCol((current) => (current + direction + 3) % 3); // 跨列环绕；空列也可落位
            return;
          }
          if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
            event.preventDefault();
            const length = summaryColumns[summaryCol].length;
            if (length > 0) {
              const direction = event.key === 'ArrowDown' ? 1 : -1;
              const current = clampSummaryRow(length);
              setSummaryRow((current + direction + length) % length); // 列内环绕
            }
            return;
          }
          if (event.key === 'Enter' && !(event.metaKey || event.ctrlKey)) {
            const column = summaryColumns[summaryCol];
            if (clampSummaryRow(column.length) < 0) {
              onKeyDown(event);
              return;
            }
            event.preventDefault();
            applySummaryActive();
            return;
          }
          if (event.key === 'Escape') {
            event.preventDefault();
            dismissedRef.current = `${trigger.kind}:${trigger.start}:${trigger.end}:${trigger.query}`;
            setTrigger(null);
            return;
          }
          onKeyDown(event);
          return;
        }
        if ((event.key === 'ArrowDown' || event.key === 'ArrowUp') && candidateCount > 0) {
          event.preventDefault();
          const direction = event.key === 'ArrowDown' ? 1 : -1;
          setActiveIndex((current) => (current + direction + candidateCount) % candidateCount);
          return;
        }
        if (event.key === 'Enter' && !(event.metaKey || event.ctrlKey)) {
          const candidate = trigger.kind === 'component'
            ? componentCandidates[activeIndex]
            : trigger.kind === 'page'
              ? pageCandidates[activeIndex]
              : promptCandidates[activeIndex];
          if (!candidate) {
            onKeyDown(event);
            return;
          }
          event.preventDefault();
          if (trigger.kind === 'component') applyComponent(candidate as ComponentReference);
          else if (trigger.kind === 'page') applyPage(candidate as PageMentionCandidate);
          else applyPrompt(candidate as Prompt);
          return;
        }
        if (event.key === 'Escape') {
          event.preventDefault();
          dismissedRef.current = `${trigger.kind}:${trigger.start}:${trigger.end}:${trigger.query}`;
          setTrigger(null);
          return;
        }
      }
      onKeyDown(event);
    };

    const handlePaste = (event: ClipboardEvent<HTMLDivElement>) => {
      event.preventDefault();
      insertPlainText(event.currentTarget, event.clipboardData.getData('text/plain'));
      syncValue();
      requestAnimationFrame(updateTrigger);
    };

    const summaryCellData = (colIndex: number, item: PageMentionCandidate | ComponentReference | Prompt) => {
      const kind = SUMMARY_COLUMNS[colIndex].kind;
      if (kind === 'page') {
        const page = item as PageMentionCandidate;
        return { key: page.slideId, name: pageDisplayName(page), desc: page.keyMessage.trim() };
      }
      if (kind === 'component') {
        const component = item as ComponentReference;
        return { key: component.id, name: component.name, desc: component.description };
      }
      const prompt = item as Prompt;
      return { key: prompt.id, name: prompt.name, desc: prompt.desc };
    };

    const summaryColumnEmptyText = (colIndex: number) => {
      const kind = SUMMARY_COLUMNS[colIndex].kind;
      const configured = kind === 'component'
        ? components.some((component) => !component.disabled)
        : kind === 'page'
          ? pages.length > 0
          : prompts.some((prompt) => !prompt.disabled);
      if (!configured) return '暂无配置';
      return kind === 'component' ? '没有匹配的组件' : kind === 'page' ? '没有匹配的页面' : '没有匹配的提示词';
    };

    const SummaryColumnIcon = ({ colIndex }: { colIndex: number }) => {
      const kind = SUMMARY_COLUMNS[colIndex].kind;
      const Icon = kind === 'page' ? GalleryThumbnails : kind === 'component' ? Blocks : NotebookText;
      return <Icon className="h-[15px] w-[15px]" strokeWidth={1.75} />;
    };

    const CommandIcon = ({ id }: { id: SlashCommandId }) => {
      const Icon = id === 'plan'
        ? ListChecks
        : id === 'grill'
          ? MessageCircleQuestion
          : id === 'chat'
            ? MessagesSquare
            : id === 'kickoff'
              ? Footprints
              : id === 'handoff'
                ? Handshake
                : id === 'commit'
                  ? GitCommitHorizontal
                  : id === 'polish'
                    ? WandSparkles
                    : id === 'model'
                      ? Cpu
                      : Crosshair;
      return <Icon className="h-[15px] w-[15px]" strokeWidth={1.75} />;
    };

    return (
      <>
        {trigger && trigger.kind === 'summary' && (
          <div
            ref={menuRef}
            className="absolute bottom-[calc(100%+4px)] left-0 right-0 z-30 grid grid-cols-3 overflow-hidden rounded-lg border border-border-strong bg-surface shadow-[0_18px_46px_rgba(31,42,55,0.2)]"
            role="grid"
            aria-label="汇总检索候选"
          >
            {SUMMARY_COLUMNS.map((column, colIndex) => {
              const items = summaryColumns[colIndex];
              const active = colIndex === summaryCol;
              const activeRow = active ? clampSummaryRow(items.length) : -1;
              return (
                <div
                  key={column.kind}
                  role="columnheader"
                  className={`flex h-[248px] min-w-0 flex-col ${colIndex > 0 ? 'border-l border-border' : ''}`}
                >
                  <div className={`flex shrink-0 items-center gap-1.5 border-b px-2.5 pb-1.5 pt-2 text-[11px] font-semibold ${
                    active ? 'border-accent/30 bg-accent/[0.04] text-accent' : 'border-panel-muted text-text-400'
                  }`}>
                    {column.label}
                    <span className={`ml-auto min-w-[18px] rounded-full px-1.5 text-center font-mono text-[10.5px] ${
                      active ? 'bg-accent-soft text-accent' : 'bg-panel-muted text-text-600'
                    }`}>
                      {items.length}
                    </span>
                  </div>
                  <div className="scrollbar-none min-h-0 flex-1 overflow-y-auto p-1">
                    {items.length === 0 ? (
                      <div className="grid h-full place-items-center px-2 text-center text-[11.5px] leading-6 text-text-400">
                        {summaryColumnEmptyText(colIndex)}
                      </div>
                    ) : items.map((item, rowIndex) => {
                      const cell = summaryCellData(colIndex, item);
                      const isActiveCell = colIndex === summaryCol && rowIndex === activeRow;
                      return (
                        <button
                          key={cell.key}
                          type="button"
                          role="gridcell"
                          aria-selected={isActiveCell}
                          data-summary-cell={`${colIndex}:${rowIndex}`}
                          onMouseEnter={() => { setSummaryCol(colIndex); setSummaryRow(rowIndex); }}
                          onMouseDown={(event) => {
                            event.preventDefault();
                            if (column.kind === 'page') applyPage(item as PageMentionCandidate);
                            else if (column.kind === 'component') applyComponent(item as ComponentReference);
                            else applyPrompt(item as Prompt);
                          }}
                          className={`grid w-full grid-cols-[20px_minmax(0,1fr)] items-center gap-2 rounded-md px-2 py-1.5 text-left ${
                            isActiveCell ? 'bg-accent-soft' : 'hover:bg-panel-muted'
                          }`}
                        >
                          <span className="grid h-6 w-5 place-items-center text-accent" aria-hidden="true">
                            <SummaryColumnIcon colIndex={colIndex} />
                          </span>
                          <span className="flex min-w-0 flex-col gap-0.5">
                            <span className="min-w-0 truncate text-xs font-bold text-text-900">{cell.name}</span>
                            {cell.desc && (
                              <span className="min-w-0 truncate text-[11px] leading-4 text-text-400" title={cell.desc}>
                                {cell.desc}
                              </span>
                            )}
                          </span>
                        </button>
                      );
                    })}
                  </div>
                </div>
              );
            })}
          </div>
        )}
        {trigger && trigger.kind === 'command' && (
          <div
            ref={menuRef}
            className="absolute bottom-[calc(100%+4px)] left-0 right-0 z-30 flex max-h-[286px] flex-col overflow-hidden rounded-lg border border-border-strong bg-surface p-1 shadow-[0_18px_46px_rgba(31,42,55,0.2)]"
            role="listbox"
            aria-label={commandLevel === 'root' ? '命令' : commandLevel === 'model' ? '选择模型' : '选择目标'}
          >
            {commandLevel !== 'root' && (
              <button
                type="button"
                onMouseDown={(event) => {
                  event.preventDefault();
                  setCommandLevel('root');
                  setActiveIndex(0);
                }}
                className="flex h-8 shrink-0 items-center gap-1.5 border-b border-border px-2 text-left text-xs font-semibold text-text-700 hover:bg-panel-muted"
              >
                <ArrowLeft className="h-3.5 w-3.5" strokeWidth={1.75} />
                {commandLevel === 'model' ? '选择模型' : '选择目标'}
              </button>
            )}
            <div className="scrollbar-none min-h-0 flex-1 overflow-y-auto">
              {candidateCount === 0 ? (
                <div className="grid h-24 place-items-center px-3 text-xs text-text-400">
                  {commandLevel === 'root' ? '无匹配命令' : '暂无可用选项'}
                </div>
              ) : commandLevel === 'root' ? (
                (['模式', '操作', '设置'] as const).map((group) => {
                  const groupCommands = commandCandidates.filter((command) => command.group === group);
                  if (groupCommands.length === 0) return null;
                  return (
                    <div key={group}>
                      <div className="px-2 pb-1 pt-2 text-[10px] font-semibold text-text-400">{group}</div>
                      {groupCommands.map((command) => {
                        const index = commandCandidates.indexOf(command);
                        const active = index === activeIndex;
                        return (
                          <button
                            key={command.id}
                            type="button"
                            role="option"
                            aria-label={command.ariaLabel}
                            aria-selected={active}
                            aria-disabled={command.disabled}
                            data-candidate-index={index}
                            onMouseEnter={() => {
                              if (!command.disabled) setActiveIndex(index);
                            }}
                            onMouseDown={(event) => {
                              event.preventDefault();
                              applyCommand(command);
                            }}
                            className={`grid min-h-9 w-full grid-cols-[20px_minmax(0,1fr)_16px] items-center gap-1.5 rounded-md px-2 py-1 text-left ${
                              command.disabled
                                ? 'cursor-not-allowed text-text-400 opacity-55'
                                : active
                                  ? 'bg-accent-soft text-text-900'
                                  : 'text-text-700 hover:bg-panel-muted'
                            }`}
                          >
                            <span className={command.disabled ? 'text-text-400' : 'text-accent'} aria-hidden="true">
                              <CommandIcon id={command.id} />
                            </span>
                            <span className="flex min-w-0 items-baseline gap-2">
                              <span className="shrink-0 font-mono text-xs font-semibold">{command.name}</span>
                              <span className="min-w-0 flex-1 truncate text-[11px] text-text-400">
                                {command.disabledReason ?? command.description}
                              </span>
                            </span>
                            {command.submenu && <ChevronRight className="h-3.5 w-3.5 text-text-400" strokeWidth={1.75} />}
                          </button>
                        );
                      })}
                    </div>
                  );
                })
              ) : commandOptions.map((option, index) => (
                <button
                  key={option.id}
                  type="button"
                  role="option"
                  aria-selected={activeIndex === index}
                  aria-disabled={option.disabled}
                  data-candidate-index={index}
                  onMouseEnter={() => {
                    if (!option.disabled) setActiveIndex(index);
                  }}
                  onMouseDown={(event) => {
                    event.preventDefault();
                    applyCommandOption(option);
                  }}
                  className={`grid min-h-9 w-full grid-cols-[minmax(0,1fr)_16px] items-center gap-2 rounded-md px-2 py-1.5 text-left ${
                    option.disabled
                      ? 'cursor-not-allowed text-text-400 opacity-55'
                      : activeIndex === index
                        ? 'bg-accent-soft text-text-900'
                        : 'text-text-700 hover:bg-panel-muted'
                  }`}
                >
                  <span className="min-w-0">
                    <span className="block truncate text-xs font-semibold">{option.label}</span>
                    {option.description && (
                      <span className="block truncate text-[10px] text-text-400">{option.description}</span>
                    )}
                  </span>
                  {option.selected && <Check className="h-3.5 w-3.5 text-accent" strokeWidth={2} />}
                </button>
              ))}
            </div>
          </div>
        )}
        {trigger && trigger.kind !== 'summary' && trigger.kind !== 'command' && (
          <div
            ref={menuRef}
            className="absolute bottom-[calc(100%+4px)] left-0 right-0 z-30 flex h-[230px] flex-col overflow-hidden rounded-lg border border-border-strong bg-surface p-1 shadow-[0_18px_46px_rgba(31,42,55,0.2)]"
            role="listbox"
            aria-label={trigger.kind === 'component' ? '组件候选' : trigger.kind === 'page' ? '页面候选' : '提示词候选'}
          >
            <div className="shrink-0 px-2 pb-1 pt-1.5 text-[11px] font-semibold text-text-400">
              {trigger.kind === 'component'
                ? '组件'
                : trigger.kind === 'page'
                  ? '页面'
                  : '提示词'}
            </div>
            <div className="scrollbar-none min-h-0 flex-1 overflow-y-auto">
              {candidateCount === 0 ? (
                <div className="grid h-full place-items-center px-3 text-xs text-text-400">
                  {!hasConfiguredCandidates
                    ? '暂无配置'
                    : trigger.kind === 'component'
                      ? '没有匹配的组件'
                      : trigger.kind === 'page'
                        ? '没有匹配的页面'
                        : '没有匹配的提示词'}
                </div>
              ) : trigger.kind === 'component' ? componentCandidates.map((component, index) => (
              <button
                key={component.id}
                type="button"
                role="option"
                aria-selected={activeIndex === index}
                data-component-option={component.id}
                data-candidate-index={index}
                onMouseEnter={() => setActiveIndex(index)}
                onMouseDown={(event) => {
                  event.preventDefault();
                  applyComponent(component);
                }}
                className={`grid min-h-[42px] w-full grid-cols-[20px_minmax(0,1fr)] items-center gap-1 rounded-md px-2 py-1.5 text-left ${
                  activeIndex === index ? 'bg-accent-soft' : 'hover:bg-panel-muted'
                }`}
              >
                <span className="grid h-6 w-5 place-items-center text-accent" aria-hidden="true">
                  <Blocks className="h-[15px] w-[15px]" strokeWidth={1.75} />
                </span>
                <span className="flex min-w-0 items-baseline gap-1.5">
                  <span className="max-w-[48%] truncate text-xs font-bold text-text-900">{component.name}</span>
                  <span className="min-w-0 flex-1 truncate text-[11px] leading-4 text-text-600">{component.description}</span>
                </span>
              </button>
            )) : trigger.kind === 'page' ? pageCandidates.map((page, index) => {
              const status = pageStatus(page);
              return (
                <button
                  key={page.slideId}
                  type="button"
                  role="option"
                  aria-selected={activeIndex === index}
                  data-page-option={page.slideId}
                  data-candidate-index={index}
                  onMouseEnter={() => setActiveIndex(index)}
                  onMouseDown={(event) => {
                    event.preventDefault();
                    applyPage(page);
                  }}
                  className={`grid min-h-[50px] w-full grid-cols-[20px_minmax(0,1fr)] items-center gap-1 rounded-md px-2 py-1.5 text-left ${
                    activeIndex === index ? 'bg-accent-soft' : 'hover:bg-panel-muted'
                  }`}
                >
                  <span className="grid h-6 w-5 place-items-center text-accent" aria-hidden="true">
                    <GalleryThumbnails className="h-[15px] w-[15px]" strokeWidth={1.75} />
                  </span>
                  <span className="flex min-w-0 flex-col gap-0.5">
                    <span className="flex min-w-0 items-baseline">
                      <span className="shrink-0 text-xs font-bold text-accent">Page {page.ordinal}</span>
                      {page.title.trim() && (
                        <>
                          <span className="mx-1 shrink-0 text-text-400">·</span>
                          <span className="min-w-0 truncate text-xs font-bold text-text-900">{page.title.trim()}</span>
                        </>
                      )}
                      {page.keyMessage.trim() && (
                        <span className="ml-2 min-w-0 flex-1 truncate text-[11px] font-normal text-text-400">
                          {page.keyMessage.trim()}
                        </span>
                      )}
                    </span>
                    <span className="flex min-w-0 items-center gap-2.5 truncate text-[11px] leading-4 text-text-400">
                      <span className="inline-flex items-center gap-1">
                        <span className={`h-1.5 w-1.5 shrink-0 rounded-full ${status.spec.dot}`} />
                        {status.spec.label}
                      </span>
                      <span className="inline-flex items-center gap-1">
                        <span className={`h-1.5 w-1.5 shrink-0 rounded-full ${status.html.dot}`} />
                        {status.html.label}
                      </span>
                    </span>
                  </span>
                </button>
              );
            }) : promptCandidates.map((prompt, index) => (
              <button
                key={prompt.id}
                type="button"
                role="option"
                aria-selected={activeIndex === index}
                data-prompt-option={prompt.id}
                data-candidate-index={index}
                onMouseEnter={() => setActiveIndex(index)}
                onMouseDown={(event) => {
                  event.preventDefault();
                  applyPrompt(prompt);
                }}
                className={`grid min-h-[42px] w-full grid-cols-[20px_minmax(0,1fr)] items-center gap-1 rounded-md px-2 py-1.5 text-left ${
                  activeIndex === index ? 'bg-accent-soft' : 'hover:bg-panel-muted'
                }`}
              >
                <span className="grid h-6 w-5 place-items-center text-accent" aria-hidden="true">
                  <NotebookText className="h-[15px] w-[15px]" strokeWidth={1.75} />
                </span>
                <span className="flex min-w-0 items-baseline gap-1.5">
                  <span className="max-w-[48%] truncate text-xs font-bold text-text-900">{prompt.name}</span>
                  <span className="min-w-0 flex-1 truncate text-[11px] leading-4 text-text-600">{prompt.desc}</span>
                </span>
              </button>
              ))}
            </div>
          </div>
        )}
        <div
          ref={editorRef}
          contentEditable={!disabled && !readOnly}
          suppressContentEditableWarning
          role="textbox"
          aria-multiline="true"
          aria-disabled={disabled}
          aria-readonly={readOnly}
          aria-busy={readOnly}
          data-placeholder={placeholder}
          data-empty={String(value.length === 0)}
          spellCheck={false}
          onFocus={() => {
            void loadPrompts().catch(() => undefined);
            void loadComponents().catch(() => undefined);
            updateTrigger();
          }}
          onBlur={() => setTrigger(null)}
          onInput={handleInput}
          onKeyDown={handleEditorKeyDown}
          onBeforeInput={(event) => {
            if (event.nativeEvent.inputType !== 'insertParagraph' && event.nativeEvent.inputType !== 'insertLineBreak') return;
            event.preventDefault();
            insertPlainText(event.currentTarget, '\n');
            syncValue();
          }}
          onPaste={handlePaste}
          onCompositionStart={() => {
            composingRef.current = true;
            onCompositionChange(true);
            setTrigger(null);
          }}
          onCompositionEnd={() => {
            composingRef.current = false;
            onCompositionChange(false);
            syncValue();
            requestAnimationFrame(updateTrigger);
          }}
          className="composer-prompt-editor max-h-32 min-h-[60px] w-full overflow-y-auto bg-transparent py-3 pl-3 pr-12 text-sm leading-5 text-text-900 focus:outline-none focus-visible:outline-none disabled:opacity-50"
        />
      </>
    );
  },
);
