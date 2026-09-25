import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { PromptComposerEditor } from './PromptComposerEditor';
import { resolveSlashCommands } from './promptMatching';
import { defaultBindings } from '../../lib/shortcuts';
import { useSnippetStore } from '../../stores/snippetStore';
import { useShortcutStore } from '../../stores/shortcutStore';

afterEach(() => { act(() => {
  useShortcutStore.setState({ bindings: defaultBindings });
  useSnippetStore.setState({ snippets: [], loaded: false });
}); });

describe('PromptComposerEditor slash command menu', () => {
  it('uses a newly configured command trigger immediately', async () => {
    render(<PromptComposerEditor value="" onChange={vi.fn()} onKeyDown={vi.fn()} onCompositionChange={vi.fn()}
      placeholder="输入命令" disabled={false} readOnly={false}
      slashCommands={resolveSlashCommands({runActive:false,emptyProject:false,operationBusy:false,hasPolishText:true})} onSlashCommand={vi.fn()} />);
    const editor = screen.getByRole('textbox');
    const type = (text: string) => act(() => {
      editor.focus(); editor.textContent = text;
      const range = document.createRange(); range.selectNodeContents(editor); range.collapse(false);
      window.getSelection()?.removeAllRanges(); window.getSelection()?.addRange(range);
      fireEvent.input(editor);
    });
    type('/');
    await screen.findByRole('listbox', {name:'命令'});
    act(() => useShortcutStore.setState({bindings:{...defaultBindings,'trigger.command':{trigger:'!'}}}));
    type('/create');
    await waitFor(() => expect(screen.queryByRole('listbox', {name:'命令'})).not.toBeInTheDocument());
    type('!create');
    await screen.findByRole('option', {name:'开发模式'});
    expect(screen.queryByRole('option', {name:'讨论模式'})).not.toBeInTheDocument();
  });
  it('renders the aligned mode icons and the selected polish icon', async () => {
    render(
      <div className="relative">
        <PromptComposerEditor
          value=""
          onChange={() => undefined}
          onKeyDown={() => undefined}
          onCompositionChange={() => undefined}
          placeholder="输入命令"
          disabled={false}
          readOnly={false}
          slashCommands={resolveSlashCommands({
            runActive: false,
            emptyProject: false,
            operationBusy: false,
            hasPolishText: true,
          })}
          onSlashCommand={vi.fn()}
        />
      </div>,
    );

    const editor = screen.getByRole('textbox');
    editor.focus();
    editor.textContent = '/';
    const range = document.createRange();
    range.selectNodeContents(editor);
    range.collapse(false);
    const selection = window.getSelection();
    selection?.removeAllRanges();
    selection?.addRange(range);
    fireEvent.input(editor);

    await waitFor(() => expect(screen.getByRole('listbox', { name: '命令' })).toBeInTheDocument());
    expect(screen.getByRole('option', { name: '计划模式' }).querySelector('svg'))
      .toHaveClass('lucide-clipboard-list');
    expect(screen.getByRole('option', { name: '盘问模式' }).querySelector('svg'))
      .toHaveClass('lucide-message-circle-question-mark');
    expect(screen.getByRole('option', { name: '启动简报' }).querySelector('svg'))
      .toHaveClass('lucide-sport-shoe');
    expect(screen.getByRole('option', { name: '交接简报' }).querySelector('svg'))
      .toHaveClass('lucide-handshake');
    expect(screen.getByRole('option', { name: '润色' }).querySelector('svg'))
      .toHaveClass('lucide-sparkles');
  });

  it('keeps the keyboard selection when ArrowDown and ArrowUp are pressed', async () => {
    render(
      <div className="relative">
        <PromptComposerEditor
          value=""
          onChange={() => undefined}
          onKeyDown={() => undefined}
          onCompositionChange={() => undefined}
          placeholder="输入命令"
          disabled={false}
          readOnly={false}
          slashCommands={resolveSlashCommands({
            runActive: false,
            emptyProject: false,
            operationBusy: false,
            hasPolishText: true,
          })}
          onSlashCommand={vi.fn()}
        />
      </div>,
    );

    const editor = screen.getByRole('textbox');
    editor.focus();
    editor.textContent = '/';
    const range = document.createRange();
    range.selectNodeContents(editor);
    range.collapse(false);
    const selection = window.getSelection();
    selection?.removeAllRanges();
    selection?.addRange(range);
    fireEvent.input(editor);

    await waitFor(() => expect(screen.getByRole('listbox', { name: '命令' })).toBeInTheDocument());
    const options = screen.getAllByRole('option');
    expect(options[0]).toHaveAttribute('aria-selected', 'true');

    fireEvent.keyDown(editor, { key: 'ArrowDown' });
    await waitFor(() => expect(options[1]).toHaveAttribute('aria-selected', 'true'));

    fireEvent.keyDown(editor, { key: 'ArrowUp' });
    await waitFor(() => expect(options[0]).toHaveAttribute('aria-selected', 'true'));
  });

  it('skips disabled commands while navigating with ArrowDown and ArrowUp', async () => {
    render(
      <div className="relative">
        <PromptComposerEditor
          value=""
          onChange={() => undefined}
          onKeyDown={() => undefined}
          onCompositionChange={() => undefined}
          placeholder="输入命令"
          disabled={false}
          readOnly={false}
          slashCommands={resolveSlashCommands({
            runActive: false,
            emptyProject: true,
            operationBusy: false,
            hasPolishText: false,
          })}
          onSlashCommand={vi.fn()}
        />
      </div>,
    );

    const editor = screen.getByRole('textbox');
    editor.focus();
    editor.textContent = '/';
    const range = document.createRange();
    range.selectNodeContents(editor);
    range.collapse(false);
    const selection = window.getSelection();
    selection?.removeAllRanges();
    selection?.addRange(range);
    fireEvent.input(editor);

    await waitFor(() => expect(screen.getByRole('listbox', { name: '命令' })).toBeInTheDocument());
    fireEvent.keyDown(editor, { key: 'ArrowDown' });
    fireEvent.keyDown(editor, { key: 'ArrowDown' });
    fireEvent.keyDown(editor, { key: 'ArrowDown' });
    fireEvent.keyDown(editor, { key: 'ArrowDown' });
    await waitFor(() => expect(screen.getByRole('option', { name: '会话命名' }))
      .toHaveAttribute('aria-selected', 'true'));

    fireEvent.keyDown(editor, { key: 'ArrowUp' });
    await waitFor(() => expect(screen.getByRole('option', { name: '计划模式' }))
      .toHaveAttribute('aria-selected', 'true'));
  });
});


describe('PromptComposerEditor snippets', () => {
  it('inserts only the payload as editable plain text using the snippet trigger', async () => {
    useSnippetStore.setState({
      loaded: true, loading: false, error: '',
      snippets: [{ id: 'phrase', name: 'Metadata name', description: 'Metadata description',
        content: 'Exact body\nSecond line', tags: ['other'], disabled: false,
        content_state: 'ready', open_url: '', created_at: 1, updated_at: 1 }],
    });
    const onChange = vi.fn();
    render(<PromptComposerEditor value="" onChange={onChange} onKeyDown={vi.fn()} onCompositionChange={vi.fn()}
      placeholder="输入" disabled={false} readOnly={false} />);
    const editor = screen.getByRole('textbox');
    act(() => {
      editor.focus(); editor.textContent = '％Metadata';
      const range = document.createRange(); range.selectNodeContents(editor); range.collapse(false);
      window.getSelection()?.removeAllRanges(); window.getSelection()?.addRange(range);
      fireEvent.input(editor);
    });
    const option = await screen.findByRole('option');
    fireEvent.mouseDown(option);
    expect(onChange).toHaveBeenLastCalledWith('Exact body\nSecond line ');
    expect(editor.textContent).toBe('Exact body\nSecond line ');
    expect(editor.textContent).not.toContain('Metadata');
  });
});
