import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { useState } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Prompt } from '../../api/types';
import { usePromptStore } from '../../stores/promptStore';
import { PromptComposerEditor } from './PromptComposerEditor';

const prompt: Prompt = {
  id: 'p1',
  key_zh: '高管摘要',
  key_en: 'executive-summary',
  value: '生成高管摘要',
  tags: ['summarize'],
  created_at: 1,
  updated_at: 1,
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

function Harness({ initial = '$sum', changed = vi.fn() }: { initial?: string; changed?: (value: string) => void }) {
  const [value, setValue] = useState(initial);
  return (
    <PromptComposerEditor
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
  });

  it('opens matching candidates and inserts an editable styled fragment plus a plain space', async () => {
    const changed = vi.fn();
    render(<Harness changed={changed} />);
    const editor = screen.getByRole('textbox');
    editor.focus();
    placeCaretAtEnd(editor);

    const option = await screen.findByRole('option', { name: /高管摘要/ });
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
});
