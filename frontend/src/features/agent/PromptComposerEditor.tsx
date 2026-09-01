import {
  forwardRef,
  useCallback,
  useEffect,
  useImperativeHandle,
  useRef,
  useState,
  type ClipboardEvent,
  type FormEvent,
  type KeyboardEvent,
} from 'react';
import { NotebookText } from 'lucide-react';
import type { Prompt } from '../../api/types';
import { usePromptStore } from '../../stores/promptStore';
import { findPromptTrigger, matchPrompts, type PromptTrigger } from './promptMatching';
import './promptComposer.css';

export interface PromptComposerEditorHandle {
  getPlainText: () => string;
  setPlainText: (value: string) => void;
  focusEnd: () => void;
  captureSelection: () => Range | null;
  restoreSelection: (range: Range | null) => void;
}

interface PromptComposerEditorProps {
  value: string;
  onChange: (value: string) => void;
  onKeyDown: (event: KeyboardEvent<HTMLDivElement>) => void;
  onCompositionChange: (composing: boolean) => void;
  placeholder: string;
  disabled: boolean;
  readOnly: boolean;
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
  return nodePlainText(root).replace(/\n$/, '');
}

function setPlainTextContent(root: HTMLElement, value: string) {
  root.replaceChildren();
  value.split('\n').forEach((line, index) => {
    if (index > 0) root.append(document.createElement('br'));
    if (line) root.append(document.createTextNode(line));
  });
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
  const fragment = document.createDocumentFragment();
  const nodes: Node[] = [];
  value.split('\n').forEach((line, index) => {
    if (index > 0) nodes.push(document.createElement('br'));
    if (line) nodes.push(document.createTextNode(line));
  });
  nodes.forEach((node) => fragment.append(node));
  const last = nodes[nodes.length - 1];
  range.insertNode(fragment);
  if (last) {
    const next = document.createRange();
    next.setStartAfter(last);
    next.collapse(true);
    selection.removeAllRanges();
    selection.addRange(next);
  }
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
  }, forwardedRef) {
    const editorRef = useRef<HTMLDivElement>(null);
    const menuRef = useRef<HTMLDivElement>(null);
    const composingRef = useRef(false);
    const dismissedRef = useRef('');
    const [trigger, setTrigger] = useState<PromptTrigger | null>(null);
    const [activeIndex, setActiveIndex] = useState(0);
    const prompts = usePromptStore((state) => state.prompts);
    const recentIds = usePromptStore((state) => state.recentIds);
    const version = usePromptStore((state) => state.version);
    const loadPrompts = usePromptStore((state) => state.load);
    const recordRecent = usePromptStore((state) => state.recordRecent);
    const candidates = trigger ? matchPrompts(prompts, trigger.query, recentIds) : [];

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
      const next = findPromptTrigger(serializeComposerText(editor), offset);
      const signature = next ? `${next.start}:${next.end}:${next.query}` : '';
      if (!next || dismissedRef.current === signature) {
        setTrigger(null);
        return;
      }
      setActiveIndex(0);
      setTrigger(next);
    }, [disabled, readOnly]);

    useImperativeHandle(forwardedRef, () => ({
      getPlainText: () => editorRef.current ? serializeComposerText(editorRef.current) : '',
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
    }, [updateTrigger, version]);

    useEffect(() => {
      menuRef.current?.querySelector<HTMLElement>(`[data-prompt-index="${activeIndex}"]`)
        ?.scrollIntoView({ block: 'nearest' });
    }, [activeIndex]);

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
      recordRecent(prompt.id);
      dismissedRef.current = '';
      setTrigger(null);
      syncValue();
      editor.focus();
    };

    const handleInput = (_event: FormEvent<HTMLDivElement>) => {
      dismissedRef.current = '';
      syncValue();
      requestAnimationFrame(updateTrigger);
    };

    const handleEditorKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
      if (trigger && !event.nativeEvent.isComposing && !composingRef.current) {
        if ((event.key === 'ArrowDown' || event.key === 'ArrowUp') && candidates.length > 0) {
          event.preventDefault();
          const direction = event.key === 'ArrowDown' ? 1 : -1;
          setActiveIndex((current) => (current + direction + candidates.length) % candidates.length);
          return;
        }
        if (event.key === 'Enter' && !(event.metaKey || event.ctrlKey) && candidates[activeIndex]) {
          event.preventDefault();
          applyPrompt(candidates[activeIndex]);
          return;
        }
        if (event.key === 'Escape') {
          event.preventDefault();
          dismissedRef.current = `${trigger.start}:${trigger.end}:${trigger.query}`;
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

    return (
      <>
        {trigger && (
          <div
            ref={menuRef}
            className="absolute bottom-[calc(100%+4px)] left-0 right-0 z-30 max-h-[280px] overflow-y-auto rounded-lg border border-border-strong bg-surface p-1 shadow-[0_18px_46px_rgba(31,42,55,0.2)]"
            role="listbox"
            aria-label="提示词候选"
          >
            <div className="px-2 pb-1 pt-1.5 text-[11px] font-semibold text-text-400">
              {trigger.query ? '匹配提示词' : '最近使用'}
            </div>
            {candidates.length === 0 ? (
              <div className="grid min-h-14 place-items-center px-3 text-xs text-text-400">
                {trigger.query ? '没有匹配的提示词' : '暂无最近使用'}
              </div>
            ) : candidates.map((prompt, index) => (
              <button
                key={prompt.id}
                type="button"
                role="option"
                aria-selected={activeIndex === index}
                data-prompt-option={prompt.id}
                data-prompt-index={index}
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
                  <span className="flex max-w-[44%] min-w-0 items-baseline gap-1 overflow-hidden">
                    <span className="truncate text-xs font-bold text-text-900">{prompt.key_zh}</span>
                    <span className="text-[11px] font-medium text-text-400">/</span>
                    <span className="truncate text-xs font-bold text-text-900">{prompt.key_en}</span>
                  </span>
                  <span className="min-w-0 flex-1 truncate text-[11px] leading-4 text-text-600">{prompt.value}</span>
                </span>
              </button>
            ))}
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
