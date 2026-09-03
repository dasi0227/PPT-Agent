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
import { Blocks, GalleryThumbnails, NotebookText } from 'lucide-react';
import type { ComponentReference, Prompt } from '../../api/types';
import { useComponentStore } from '../../stores/componentStore';
import { usePromptStore } from '../../stores/promptStore';
import {
  findComponentTrigger,
  findPageTrigger,
  findPromptTrigger,
  findSlashTrigger,
  MAX_COMPONENT_MENTIONS,
  MAX_PAGE_MENTIONS,
  matchComponents,
  matchPages,
  matchPrompts,
  pageDisplayName,
  type PageMentionCandidate,
  type PromptTrigger,
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

type ComposerTrigger = PromptTrigger & { kind: 'prompt' | 'component' | 'page' | 'slash' };

const SLASH_COLUMNS = [
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
  }, forwardedRef) {
    const editorRef = useRef<HTMLDivElement>(null);
    const menuRef = useRef<HTMLDivElement>(null);
    const composingRef = useRef(false);
    const dismissedRef = useRef('');
    const [trigger, setTrigger] = useState<ComposerTrigger | null>(null);
    const [activeIndex, setActiveIndex] = useState(0);
    const [slashCol, setSlashCol] = useState(0);
    const [slashRow, setSlashRow] = useState(0);
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
    const isSlash = trigger?.kind === 'slash';
    const promptCandidates = trigger && (trigger.kind === 'prompt' || isSlash) ? matchPrompts(prompts, trigger.query) : [];
    const componentCandidates = trigger && (trigger.kind === 'component' || isSlash) ? matchComponents(components, trigger.query) : [];
    const pageCandidates = trigger && (trigger.kind === 'page' || isSlash) ? matchPages(pages, trigger.query) : [];
    // 汇总面板三列（页面 · 组件 · 提示词），列内候选沿用各自匹配规则
    const slashColumns = [pageCandidates, componentCandidates, promptCandidates];
    const clampSlashRow = (length: number) => (length ? Math.max(0, Math.min(slashRow, length - 1)) : -1);
    const candidateCount = trigger?.kind === 'component'
      ? componentCandidates.length
      : trigger?.kind === 'page'
        ? pageCandidates.length
        : promptCandidates.length;
    const hasConfiguredCandidates = trigger?.kind === 'component'
      ? components.some((component) => !component.disabled)
      : trigger?.kind === 'page'
        ? pages.length > 0
        : prompts.some((prompt) => !prompt.disabled);

    const updateTrigger = useCallback(() => {
      const editor = editorRef.current;
      if (!editor || disabled || readOnly || composingRef.current || document.activeElement !== editor) {
        setTrigger(null);
        return;
      }
      const offset = caretOffset(editor);
      if (offset == null) {
        setTrigger(null);
        return;
      }
      const text = serializeComposerText(editor);
      const promptTrigger = findPromptTrigger(text, offset);
      const componentTrigger = findComponentTrigger(text, offset);
      const pageTrigger = findPageTrigger(text, offset);
      const slashTrigger = findSlashTrigger(text, offset);
      const next: ComposerTrigger | null = promptTrigger
        ? { ...promptTrigger, kind: 'prompt' }
        : componentTrigger
          ? { ...componentTrigger, kind: 'component' }
          : pageTrigger
            ? { ...pageTrigger, kind: 'page' }
            : slashTrigger
              ? { ...slashTrigger, kind: 'slash' }
              : null;
      const signature = next ? `${next.kind}:${next.start}:${next.end}:${next.query}` : '';
      if (!next || dismissedRef.current === signature) {
        setTrigger(null);
        return;
      }
      setActiveIndex(0);
      // 首次进入汇总面板：高亮第一个非空列的首项
      if (next.kind === 'slash' && trigger?.kind !== 'slash') {
        const cols = [
          matchPages(pages, next.query).length,
          matchComponents(components, next.query).length,
          matchPrompts(prompts, next.query).length,
        ];
        const firstNonEmpty = cols.findIndex((length) => length > 0);
        setSlashCol(firstNonEmpty < 0 ? 0 : firstNonEmpty);
        setSlashRow(0);
      }
      setTrigger(next);
    }, [components, disabled, pages, prompts, readOnly, trigger]);

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
      if (isSlash) {
        menuRef.current?.querySelector<HTMLElement>(`[data-slash-cell="${slashCol}:${clampSlashRow(slashColumns[slashCol].length)}"]`)
          ?.scrollIntoView({ block: 'nearest' });
        return;
      }
      menuRef.current?.querySelector<HTMLElement>(`[data-candidate-index="${activeIndex}"]`)
        ?.scrollIntoView({ block: 'nearest' });
    }, [activeIndex, isSlash, slashCol, slashRow, slashColumns]);

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
      if (!editor || !trigger || (trigger.kind !== 'component' && trigger.kind !== 'slash')) return;
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
      if (!editor || !trigger || (trigger.kind !== 'page' && trigger.kind !== 'slash')) return;
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

    // 汇总面板选中：按当前列分派到对应的插入逻辑
    const applySlashActive = () => {
      const column = slashColumns[slashCol];
      const row = clampSlashRow(column.length);
      if (row < 0) return; // 空列：Enter 无效
      const kind = SLASH_COLUMNS[slashCol].kind;
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
        if (trigger.kind === 'slash') {
          if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') {
            event.preventDefault();
            const direction = event.key === 'ArrowRight' ? 1 : -1;
            setSlashCol((current) => (current + direction + 3) % 3); // 跨列环绕；空列也可落位
            return;
          }
          if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
            event.preventDefault();
            const length = slashColumns[slashCol].length;
            if (length > 0) {
              const direction = event.key === 'ArrowDown' ? 1 : -1;
              const current = clampSlashRow(length);
              setSlashRow((current + direction + length) % length); // 列内环绕
            }
            return;
          }
          if (event.key === 'Enter' && !(event.metaKey || event.ctrlKey)) {
            const column = slashColumns[slashCol];
            if (clampSlashRow(column.length) < 0) {
              onKeyDown(event);
              return;
            }
            event.preventDefault();
            applySlashActive();
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

    const slashCellData = (colIndex: number, item: PageMentionCandidate | ComponentReference | Prompt) => {
      const kind = SLASH_COLUMNS[colIndex].kind;
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

    const slashColumnEmptyText = (colIndex: number) => {
      const kind = SLASH_COLUMNS[colIndex].kind;
      const configured = kind === 'component'
        ? components.some((component) => !component.disabled)
        : kind === 'page'
          ? pages.length > 0
          : prompts.some((prompt) => !prompt.disabled);
      if (!configured) return '暂无配置';
      return kind === 'component' ? '没有匹配的组件' : kind === 'page' ? '没有匹配的页面' : '没有匹配的提示词';
    };

    const SlashColumnIcon = ({ colIndex }: { colIndex: number }) => {
      const kind = SLASH_COLUMNS[colIndex].kind;
      const Icon = kind === 'page' ? GalleryThumbnails : kind === 'component' ? Blocks : NotebookText;
      return <Icon className="h-[15px] w-[15px]" strokeWidth={1.75} />;
    };

    return (
      <>
        {trigger && trigger.kind === 'slash' && (
          <div
            ref={menuRef}
            className="absolute bottom-[calc(100%+4px)] left-0 right-0 z-30 grid grid-cols-3 overflow-hidden rounded-lg border border-border-strong bg-surface shadow-[0_18px_46px_rgba(31,42,55,0.2)]"
            role="grid"
            aria-label="汇总检索候选"
          >
            {SLASH_COLUMNS.map((column, colIndex) => {
              const items = slashColumns[colIndex];
              const active = colIndex === slashCol;
              const activeRow = active ? clampSlashRow(items.length) : -1;
              return (
                <div
                  key={column.kind}
                  role="columnheader"
                  className={`flex h-[248px] min-w-0 flex-col ${colIndex > 0 ? 'border-l border-border' : ''}`}
                >
                  <div className={`flex shrink-0 items-center gap-1.5 border-b px-2.5 pb-1.5 pt-2 text-[11px] font-semibold ${
                    active ? 'border-accent/30 bg-accent/[0.04] text-accent' : 'border-panel-muted text-text-400'
                  }`}>
                    <SlashColumnIcon colIndex={colIndex} />
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
                        {slashColumnEmptyText(colIndex)}
                      </div>
                    ) : items.map((item, rowIndex) => {
                      const cell = slashCellData(colIndex, item);
                      const isActiveCell = colIndex === slashCol && rowIndex === activeRow;
                      return (
                        <button
                          key={cell.key}
                          type="button"
                          role="gridcell"
                          aria-selected={isActiveCell}
                          data-slash-cell={`${colIndex}:${rowIndex}`}
                          onMouseEnter={() => { setSlashCol(colIndex); setSlashRow(rowIndex); }}
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
                            <SlashColumnIcon colIndex={colIndex} />
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
        {trigger && trigger.kind !== 'slash' && (
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
