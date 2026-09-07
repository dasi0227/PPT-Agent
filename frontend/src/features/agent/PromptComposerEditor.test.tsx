import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { PromptComposerEditor } from './PromptComposerEditor';
import { resolveSlashCommands } from './promptMatching';

describe('PromptComposerEditor slash command menu', () => {
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
      .toHaveClass('lucide-list-checks');
    expect(screen.getByRole('option', { name: '审问模式' }).querySelector('svg'))
      .toHaveClass('lucide-message-circle-question-mark');
    expect(screen.getByRole('option', { name: '启动简报' }).querySelector('svg'))
      .toHaveClass('lucide-footprints');
    expect(screen.getByRole('option', { name: '交接简报' }).querySelector('svg'))
      .toHaveClass('lucide-handshake');
    expect(screen.getByRole('option', { name: '润色' }).querySelector('svg'))
      .toHaveClass('lucide-wand-sparkles');
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
    await waitFor(() => expect(screen.getByRole('option', { name: '切换模型' }))
      .toHaveAttribute('aria-selected', 'true'));

    fireEvent.keyDown(editor, { key: 'ArrowUp' });
    await waitFor(() => expect(screen.getByRole('option', { name: '聊天模式' }))
      .toHaveAttribute('aria-selected', 'true'));
  });
});
