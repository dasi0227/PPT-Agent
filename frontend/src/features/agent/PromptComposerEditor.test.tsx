import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { createRef, useState, type RefObject } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { ComponentReference, Prompt } from '../../api/types';
import { useComponentStore } from '../../stores/componentStore';
import { usePromptStore } from '../../stores/promptStore';
import { PromptComposerEditor, type PromptComposerEditorHandle } from './PromptComposerEditor';
import type { PageMentionCandidate } from './promptMatching';

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

const page: PageMentionCandidate = {
  slideId: 'sli_b',
  ordinal: 2,
  title: '融资历程',
  keyMessage: '三轮融资累计 2.4 亿',
  specState: 'ready',
  htmlState: 'spec_stale',
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
  pages = [],
}: {
  initial?: string;
  changed?: (value: string) => void;
  editorRef?: RefObject<PromptComposerEditorHandle>;
  pages?: PageMentionCandidate[];
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
      pages={pages}
    />
  );
}

describe('PromptComposerEditor', () => {
  beforeEach(() => {
    localStorage.clear();
    usePromptStore.setState({
      prompts: [prompt],
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

  it('inserts page fragments with statuses and serializes stable slide ids', async () => {
    const editorRef = createRef<PromptComposerEditorHandle>();
    render(<Harness initial="@融" editorRef={editorRef} pages={[page]} />);
    const editor = screen.getByRole('textbox');
    editor.focus();
    placeCaretAtEnd(editor);

    const option = await screen.findByRole('option', { name: /Page 2.*融资历程/ });
    expect(option).toHaveTextContent(page.keyMessage);
    expect(option).toHaveTextContent('设计稿已就绪');
    expect(option).toHaveTextContent('幻灯片待更新');
    fireEvent.mouseDown(option);

    expect(editor.querySelector('[data-slide-id="sli_b"]')).toHaveTextContent('Page 2 · 融资历程');
    expect(editorRef.current?.getPlainText()).toBe('Page 2 · 融资历程 ');
    expect(editorRef.current?.getSubmitText()).toBe('Page 2 · 融资历程⟨sli_b⟩ ');
    expect(editorRef.current?.getMentionedSlideIds()).toEqual(['sli_b']);
  });

  it('opens page candidates when @ is the first character in an empty editor', async () => {
    render(<Harness initial="" pages={[page]} />);
    const editor = screen.getByRole('textbox');
    editor.focus();
    editor.append(document.createTextNode('@'));
    placeCaretAtEnd(editor);
    fireEvent.input(editor);

    expect(await screen.findByRole('option', { name: /Page 2.*融资历程/ })).toBeInTheDocument();
  });

  it('preserves a leading newline and opens page candidates after it', async () => {
    const editorRef = createRef<PromptComposerEditorHandle>();
    render(<Harness initial="" editorRef={editorRef} pages={[page]} />);
    const editor = screen.getByRole('textbox');
    editor.focus();
    editor.append(document.createTextNode('\n@'));
    placeCaretAtEnd(editor);
    fireEvent.input(editor);

    expect(editorRef.current?.getPlainText()).toBe('\n@');
    expect(await screen.findByRole('option', { name: /Page 2.*融资历程/ })).toBeInTheDocument();
  });

  it('opens an empty configuration menu for @, #, $, and ¥', async () => {
    usePromptStore.setState({ prompts: [], loaded: true, version: 2 });
    useComponentStore.setState({ components: [], loaded: true, version: 2 });

    for (const [trigger, title] of [['@', '页面'], ['#', '组件'], ['$', '提示词'], ['¥', '提示词']]) {
      const view = render(<Harness initial={trigger} pages={[]} />);
      const editor = screen.getByRole('textbox');
      editor.focus();
      placeCaretAtEnd(editor);

      const menu = await screen.findByRole('listbox');
      expect(menu).toHaveClass('h-[230px]', 'overflow-hidden');
      expect(menu.lastElementChild).toHaveClass('flex-1', 'overflow-y-auto', 'scrollbar-none');
      expect(await screen.findByText(title)).toBeInTheDocument();
      expect(await screen.findByText('暂无配置')).toBeInTheDocument();
      view.unmount();
    }
  });

  it('refreshes an unsent page fragment after reorder or rename and marks deletion invalid', async () => {
    const editorRef = createRef<PromptComposerEditorHandle>();
    const { rerender } = render(<Harness initial="@融" editorRef={editorRef} pages={[page]} />);
    const editor = screen.getByRole('textbox');
    editor.focus();
    placeCaretAtEnd(editor);
    fireEvent.mouseDown(await screen.findByRole('option', { name: /Page 2.*融资历程/ }));

    rerender(<Harness initial="@融" editorRef={editorRef} pages={[{ ...page, ordinal: 4, title: '融资进展' }]} />);
    await waitFor(() => expect(editor.querySelector('[data-slide-id="sli_b"]')).toHaveTextContent('Page 4 · 融资进展'));

    rerender(<Harness initial="@融" editorRef={editorRef} pages={[]} />);
    await waitFor(() => expect(editor.querySelector('[data-slide-id="sli_b"]')).toHaveClass('composer-page-fragment-invalid'));
    expect(editorRef.current?.getMentionedSlideIds()).toEqual(['sli_b']);
  });

  it('opens a three-column panel for / and pre-selects the first non-empty column', async () => {
    render(<Harness initial="/" pages={[page]} />);
    const editor = screen.getByRole('textbox');
    editor.focus();
    placeCaretAtEnd(editor);

    const grid = await screen.findByRole('grid', { name: '汇总检索候选' });
    expect(within(grid).getByText('Page 2 · 融资历程')).toBeInTheDocument();
    expect(within(grid).getByText(component.name)).toBeInTheDocument();
    expect(within(grid).getByText(prompt.name)).toBeInTheDocument();

    const pageCell = grid.querySelector('[data-slash-cell="0:0"]');
    expect(pageCell).toHaveAttribute('aria-selected', 'true');
  });

  it('navigates columns with arrows and inserts the active cell on Enter', async () => {
    const changed = vi.fn();
    render(<Harness initial="/" changed={changed} pages={[page]} />);
    const editor = screen.getByRole('textbox');
    editor.focus();
    placeCaretAtEnd(editor);

    const grid = await screen.findByRole('grid', { name: '汇总检索候选' });
    fireEvent.keyDown(editor, { key: 'ArrowRight' });
    await waitFor(() => expect(grid.querySelector('[data-slash-cell="1:0"]')).toHaveAttribute('aria-selected', 'true'));

    fireEvent.keyDown(editor, { key: 'Enter' });
    await waitFor(() => expect(changed).toHaveBeenLastCalledWith('能力卡片 '));
    expect(editor.querySelector('[data-component-name="能力卡片"]')).toHaveClass('composer-component-fragment');
    expect(screen.queryByRole('grid')).not.toBeInTheDocument();
  });
});
