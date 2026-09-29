import { installTagDictionaryFixture } from '../../testSupport/resourceTags';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest';
import { PromptComposerEditor } from './PromptComposerEditor';
import { MODE_META } from './modeMeta';
import { resolveSlashCommands } from './promptMatching';
import { defaultBindings } from '../../lib/shortcuts';
import { useSnippetStore } from '../../stores/snippetStore';
import { useShortcutStore } from '../../stores/shortcutStore';

beforeEach(installTagDictionaryFixture);

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
  it('shows one mode entry and opens the mode submenu', async () => {
    const onModeOption = vi.fn();
    const onSlashCommand = vi.fn();
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
          modeOptions={[
            { id: 'execute', label: '开发', icon: MODE_META.execute.icon, selected: true },
            { id: 'chat', label: '讨论', icon: MODE_META.chat.icon },
            { id: 'grill', label: '盘问', icon: MODE_META.grill.icon },
            { id: 'plan', label: '计划', icon: MODE_META.plan.icon },
          ]}
          modelOptions={[{ id: 'gpt-6', label: 'GPT 6', description: 'gpt-6', provider: 'openai', selected: true }]}
          onSlashCommand={onSlashCommand}
          onModeOption={onModeOption}
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
    expect(document.querySelector('[data-command-group-heading]')).toHaveTextContent('命令');
    expect(screen.getByRole('option', { name: '切换模式' })).toBeInTheDocument();
    expect(screen.queryByRole('option', { name: '计划模式' })).not.toBeInTheDocument();
    expect(screen.queryByRole('option', { name: '盘问模式' })).not.toBeInTheDocument();
    expect(screen.getByRole('option', { name: '交接简报' }).querySelector('svg'))
      .toHaveClass('lucide-handshake');
    expect(screen.getByRole('option', { name: '润色' }).querySelector('svg'))
      .toHaveClass('lucide-sparkles');
    fireEvent.mouseDown(screen.getByRole('option', { name: '切换模式' }));
    await screen.findByRole('listbox', { name: '选择模式' });
    expect(screen.getAllByRole('option')).toHaveLength(4);
    expect(screen.getByRole('option', { name: /计划/ }).querySelector('svg'))
      .toHaveClass('lucide-clipboard-list');
    fireEvent.mouseDown(screen.getByRole('option', { name: /计划/ }));
    expect(onModeOption).toHaveBeenCalledWith('plan');

    for (const [name, id] of [['plan', 'plan'], ['chat', 'chat']] as const) {
      editor.textContent = `/${name}`;
      const directRange = document.createRange();
      directRange.selectNodeContents(editor);
      directRange.collapse(false);
      selection?.removeAllRanges();
      selection?.addRange(directRange);
      fireEvent.input(editor);
      await screen.findByRole('option', { name: id === 'plan' ? '计划模式' : '讨论模式' });
      fireEvent.keyDown(editor, { key: 'Enter' });
      expect(onSlashCommand).toHaveBeenLastCalledWith(id);
    }

    editor.textContent = '/model';
    const modelRange = document.createRange();
    modelRange.selectNodeContents(editor);
    modelRange.collapse(false);
    selection?.removeAllRanges();
    selection?.addRange(modelRange);
    fireEvent.input(editor);
    fireEvent.mouseDown(await screen.findByRole('option', { name: '切换模型' }));
    await screen.findByRole('listbox', { name: '选择模型' });
    expect(screen.getByRole('option', { name: /GPT 6/ }).querySelector('img'))
      .toHaveAttribute('src', '/model-logos/openai.svg');
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
    await waitFor(() => expect(screen.getByRole('option', { name: '提交' })).toHaveAttribute('aria-selected', 'true'));

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
    await waitFor(() => expect(screen.getByRole('option', { name: '会话命名' }))
      .toHaveAttribute('aria-selected', 'true'));

    fireEvent.keyDown(editor, { key: 'ArrowUp' });
    await waitFor(() => expect(screen.getByRole('option', { name: '主页' }))
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
