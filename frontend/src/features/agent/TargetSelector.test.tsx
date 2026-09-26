import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { TargetSelector } from './TargetSelector';

const base = {
  selection: 'current_page' as const,
  selectedSlideIds: [],
  selectedSectionIds: [],
  pages: [{ id: 'sli_1', ordinal: 1, title: '封面' }, { id: 'sli_2', ordinal: 2, title: '结论' }],
  sections: [{ id: 'sec_1', title: '开场', pageCount: 2 }],
  onSelectionChange: vi.fn(),
  onToggleSlide: vi.fn(),
  onToggleSection: vi.fn(),
};

describe('TargetSelector', () => {
  it('does not open the menu while disabled', () => {
    render(<TargetSelector {...base} disabled />);
    const trigger = screen.getByRole('button', { name: '范围：当前页' });
    expect(trigger).toBeDisabled();

    fireEvent.pointerDown(trigger, { button: 0, ctrlKey: false });
    expect(screen.queryByRole('menu')).not.toBeInTheDocument();
  });

  it('renders page-only entries and opens a second-level list', () => {
    render(<TargetSelector {...base} />);
    const trigger = screen.getByRole('button', { name: '范围：当前页' });
    fireEvent.pointerDown(trigger, { button: 0, ctrlKey: false });
    fireEvent.click(trigger);
    expect(screen.getAllByRole('menuitemradio')).toHaveLength(4);
    expect(screen.queryByText('修改对象')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('menuitemradio', { name: '自选页' }));
    expect(screen.queryByRole('menuitemradio')).not.toBeInTheDocument();
    expect(screen.getByRole('listbox', { name: '选择页面' })).toBeInTheDocument();
    expect(base.onSelectionChange).toHaveBeenCalledWith('custom_pages');
  });

  it('keeps the trigger simple and shows page ordinals and titles in the custom list', () => {
    render(<TargetSelector {...base} selection="custom_pages" selectedSlideIds={['sli_2']} />);
    const trigger = screen.getByRole('button', { name: '范围：自选页' });
    expect(trigger).toHaveTextContent('自选页');
    expect(trigger).not.toHaveTextContent('2 页');
    fireEvent.pointerDown(trigger, { button: 0, ctrlKey: false });
    fireEvent.click(trigger);
    fireEvent.click(screen.getByRole('menuitemradio', { name: '自选页' }));
    expect(screen.getByText('封面')).toBeInTheDocument();
    expect(screen.getByText('结论')).toBeInTheDocument();
    expect(screen.getByText('第 1 页')).toBeInTheDocument();
    expect(screen.getByText('第 2 页')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('option', { name: /封面/ }));
    expect(base.onToggleSlide).toHaveBeenCalledWith('sli_1');
    expect(screen.getByRole('listbox')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '返回范围选择' }));
    expect(screen.getAllByRole('menuitemradio')).toHaveLength(4);
  });

  it('labels custom sections by chapter order instead of page count', () => {
    render(<TargetSelector {...base} selection="custom_sections" />);
    const trigger = screen.getByRole('button', { name: '范围：自选章' });
    fireEvent.pointerDown(trigger, { button: 0, ctrlKey: false });
    fireEvent.click(trigger);
    fireEvent.click(screen.getByRole('menuitemradio', { name: '自选章' }));
    expect(screen.getByText('第 1 章')).toBeInTheDocument();
    expect(screen.queryByText('2 页')).not.toBeInTheDocument();
  });

  it('closes the custom window first and the selector second with Escape', async () => {
    render(<TargetSelector {...base} selection="custom_pages" selectedSlideIds={['sli_2']} />);
    const trigger = screen.getByRole('button', { name: '范围：自选页' });
    trigger.focus();
    fireEvent.pointerDown(trigger, { button: 0, ctrlKey: false });
    fireEvent.click(trigger);
    fireEvent.click(screen.getByRole('menuitemradio', { name: '自选页' }));
    const menu = screen.getByRole('menu');

    fireEvent.keyDown(menu, { key: 'Escape' });
    expect(screen.queryByText('封面')).not.toBeInTheDocument();
    expect(screen.getByRole('menu')).toBeInTheDocument();

    fireEvent.keyDown(menu, { key: 'Escape' });
    await waitFor(() => expect(screen.queryByRole('menu')).not.toBeInTheDocument());
    expect(trigger).not.toHaveFocus();
  });

  it('keeps the selector open in an empty project and disables unavailable targets', () => {
    render(<TargetSelector {...base} selection="all_pages" pages={[]} sections={[]} emptyProject />);
    const trigger = screen.getByRole('button', { name: '范围：全部页' });
    fireEvent.pointerDown(trigger, { button: 0, ctrlKey: false });
    fireEvent.click(trigger);
    expect(screen.getByRole('menuitemradio', { name: '当前页' })).toHaveAttribute('data-disabled');
    expect(screen.getByRole('menuitemradio', { name: '自选页' })).toHaveAttribute('data-disabled');
    expect(screen.getByRole('menuitemradio', { name: '自选章' })).toHaveAttribute('data-disabled');
    expect(screen.getByRole('menuitemradio', { name: '全部页' })).not.toHaveAttribute('data-disabled');
  });
});
