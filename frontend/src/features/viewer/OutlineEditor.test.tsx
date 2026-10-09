import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Outline } from '../../api/types';
import { useDeckStore } from '../../stores/deckStore';
import { OutlineEditor } from './OutlineEditor';

const value: Outline = { sections: [{ id: 'sec_a', title: '封面', purpose: '说明主题', subsections: [], slides: [{ id: 'sli_a', title: '第一页' }] }] };

beforeEach(() => {
  useDeckStore.setState({ contentMode: 'preview', sourceBlocked: false });
  HTMLElement.prototype.scrollIntoView = vi.fn();
});

describe('outline inline editing', () => {
  it('cancels a new page, chapter and first subsection without submitting or losing the direct pages', async () => {
    const user = userEvent.setup();
    const commit = vi.fn().mockResolvedValue(undefined);
    render(<OutlineEditor value={value} version="one" commit={commit} />);
    await user.click(screen.getByRole('button', { name: '封面操作' }));
    await user.click(await screen.findByRole('menuitem', { name: '新增页面' }));
    expect(screen.getByRole('textbox', { name: '页面名称' })).toHaveValue('新页面');
    await user.click(screen.getByRole('button', { name: '取消编辑' }));
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '新增章节' }));
    expect(screen.getByRole('textbox', { name: '章节名称' })).toHaveValue('新章节');
    await user.keyboard('{Escape}');
    expect(screen.queryByText('新章节')).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '封面操作' }));
    await user.click(await screen.findByRole('menuitem', { name: '新增小节' }));
    expect(screen.getByRole('textbox', { name: '小节名称' })).toHaveValue('新小节');
    expect(screen.getByRole('button', { name: '第一页' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '取消编辑' }));
    expect(screen.queryByText('新小节')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: '第一页' })).toBeInTheDocument();
    expect(commit).not.toHaveBeenCalled();
    expect(value.sections[0].subsections).toEqual([]);
  });

  it('keeps the input after a failed save and prevents confirming a stale edit', async () => {
    const user = userEvent.setup();
    const commit = vi.fn().mockRejectedValue(new Error('保存失败'));
    const { rerender } = render(<OutlineEditor value={value} version="one" commit={commit} />);
    await user.click(screen.getByRole('button', { name: '封面操作' }));
    await user.click(await screen.findByRole('menuitem', { name: '编辑章节' }));
    const name = screen.getByRole('textbox', { name: '章节名称' });
    await user.clear(name); await user.type(name, '新封面{Enter}');
    expect(await screen.findByText('保存失败')).toBeInTheDocument();
    expect(name).toHaveValue('新封面');
    expect(commit).toHaveBeenCalledWith({ type: 'update', id: 'sec_a', title: '新封面', purpose: '说明主题' }, 'one');
    rerender(<OutlineEditor value={value} version="two" commit={commit} />);
    expect(screen.getByRole('button', { name: '确认编辑' })).toBeDisabled();
    fireEvent.keyDown(name, { key: 'Enter' });
    expect(commit).toHaveBeenCalledTimes(1);
    await user.click(screen.getByRole('button', { name: '取消编辑' }));
    expect(screen.getByRole('button', { name: '封面' })).toBeInTheDocument();
  });

  it('uses the shared read-only JSON view and blocks switching while editing', async () => {
    const user = userEvent.setup();
    const commit = vi.fn();
    const { container } = render(<OutlineEditor value={value} version="one" commit={commit} />);
    await user.click(screen.getByRole('button', { name: '封面', exact: true }));
    expect(useDeckStore.getState().sourceBlocked).toBe(true);
    act(() => useDeckStore.getState().setContentMode('source'));
    expect(useDeckStore.getState().contentMode).toBe('preview');
    await user.click(screen.getByRole('button', { name: '取消编辑' }));
    await waitFor(() => expect(useDeckStore.getState().sourceBlocked).toBe(false));
    act(() => useDeckStore.getState().setContentMode('source'));
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
    expect(container.querySelector('code')).toHaveTextContent('"title": "封面"');
    expect(commit).not.toHaveBeenCalled();
  });
});
