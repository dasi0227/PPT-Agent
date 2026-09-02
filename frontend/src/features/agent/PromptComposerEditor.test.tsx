import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { createRef, useState, type RefObject } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { ComponentReference, Prompt } from '../../api/types';
import { useComponentStore } from '../../stores/componentStore';
import { usePromptStore } from '../../stores/promptStore';
import { PromptComposerEditor, type PromptComposerEditorHandle } from './PromptComposerEditor';

const prompt: Prompt = {
  id: 'p1',
  name: '高管摘要 / Executive Summary',
  desc: '提炼核心结论',
  value: '生成高管摘要',
  tags: ['deliverable'],
  disabled: false,
  created_at: 1,
  updated_at: 1,
};

const component: ComponentReference = {
  id: 'feature-card',
  name: '能力卡片',
  description: '展示核心能力',
  tags: ['card'],
  disabled: false,
  open_url: 'vscode://file/component',
};

function placeCaretAtEnd(element: HTMLElement) {
  const range = document.createRange();
  range.selectNodeContents(element);
  range.collapse(false);
  const selection = window.getSelection();
  selection?.removeAllRanges();
  selection?.addRange(range);
  fireEvent(document, new Event('selectionchange'));
}

function Harness({
  initial = '$sum',
  changed = vi.fn(),
  editorRef,
}: {
  initial?: string;
  changed?: (value: string) => void;
  editorRef?: RefObject<PromptComposerEditorHandle>;
}) {
  const [value, setValue] = useState(initial);
  return (
    <PromptComposerEditor
      ref={editorRef}
      value={value}
      onChange={(next) => {
        setValue(next);
        changed(next);
      }}
      onKeyDown={() => undefined}
      onCompositionChange={() => undefined}
      placeholder="输入"
      disabled={false}
      readOnly={false}
    />
  );
}

describe('PromptComposerEditor', () => {
  beforeEach(() => {
    localStorage.clear();
    usePromptStore.setState({
      prompts: [prompt],
      recentIds: [],
      loading: false,
      loaded: true,
      error: '',
      version: 1,
    });
    useComponentStore.setState({
      components: [component],
      loading: false,
      loaded: true,
      error: '',
      version: 1,
    });
  });

  it('opens matching candidates and inserts an editable styled fragment plus a plain space', async () => {
    const changed = vi.fn();
    render(<Harness changed={changed} />);
    const editor = screen.getByRole('textbox');
    editor.focus();
    placeCaretAtEnd(editor);

    const option = await screen.findByRole('option', { name: /高管摘要/ });
    expect(option).toHaveTextContent(prompt.desc);
    expect(option).not.toHaveTextContent(prompt.value);
    fireEvent.mouseDown(option);

    await waitFor(() => expect(changed).toHaveBeenLastCalledWith('生成高管摘要 '));
    const fragment = editor.querySelector('[data-prompt-id="p1"]');
    expect(fragment).toHaveClass('composer-prompt-fragment');
    expect(fragment).not.toHaveAttribute('contenteditable');
    expect(editor.textContent).toBe('生成高管摘要 ');
    expect(usePromptStore.getState().recentIds[0]).toBe('p1');
  });

  it('does not trigger after punctuation and closes on Escape', async () => {
    const { rerender } = render(<Harness initial="正文，$sum" />);
    const editor = screen.getByRole('textbox');
    editor.focus();
    placeCaretAtEnd(editor);
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();

    rerender(<Harness key="valid" initial="$sum" />);
    const nextEditor = screen.getByRole('textbox');
    nextEditor.focus();
    placeCaretAtEnd(nextEditor);
    expect(await screen.findByRole('listbox')).toBeInTheDocument();
    fireEvent.keyDown(nextEditor, { key: 'Escape' });
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
  });

  it('opens # candidates and inserts a component name fragment plus a plain space', async () => {
    const changed = vi.fn();
    const editorRef = createRef<PromptComposerEditorHandle>();
    render(<Harness initial="#能力" changed={changed} editorRef={editorRef} />);
    const editor = screen.getByRole('textbox');
    editor.focus();
    placeCaretAtEnd(editor);

    const option = await screen.findByRole('option', { name: /能力卡片/ });
    expect(option).toHaveTextContent(component.description);
    fireEvent.mouseDown(option);

    await waitFor(() => expect(changed).toHaveBeenLastCalledWith('能力卡片 '));
    const fragment = editor.querySelector('[data-component-name="能力卡片"]');
    expect(fragment).toHaveClass('composer-component-fragment');
    expect(fragment).not.toHaveAttribute('contenteditable');
    expect(editor.textContent).toBe('能力卡片 ');
    expect(editorRef.current?.getComponentNames()).toEqual(['能力卡片']);
    editorRef.current?.setPlainText('润色后的普通文本');
    expect(editorRef.current?.getComponentNames()).toEqual([]);
  });

  it('collects component names in document order with deduplication and an eight-item cap', () => {
    const editorRef = createRef<PromptComposerEditorHandle>();
    render(<Harness initial="" editorRef={editorRef} />);
    const editor = screen.getByRole('textbox');
    ['A', 'B', 'A', 'C', 'D', 'E', 'F', 'G', 'H', 'I'].forEach((name) => {
      const fragment = document.createElement('span');
      fragment.dataset.componentName = name;
      fragment.textContent = name;
      editor.append(fragment);
    });

    expect(editorRef.current?.getComponentNames()).toEqual(['A', 'B', 'C', 'D', 'E', 'F', 'G', 'H']);
  });
});
