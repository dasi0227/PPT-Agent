import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { TargetSelector } from './TargetSelector';

const base = {
  object: 'presentation' as const,
  selection: 'current_page' as const,
  selectedSlideIds: [],
  selectedSectionIds: [],
  pages: [{ id: 'sli_1', ordinal: 1, title: '封面' }, { id: 'sli_2', ordinal: 2, title: '结论' }],
  sections: [{ id: 'sec_1', title: '开场', pageCount: 2 }],
  onObjectChange: vi.fn(),
  onSelectionChange: vi.fn(),
  onToggleSlide: vi.fn(),
  onToggleSection: vi.fn(),
};

describe('TargetSelector', () => {
  it('renders the compact two-row selector', () => {
    render(<TargetSelector {...base} />);
    const trigger = screen.getByRole('button', { name: '范围：当前页 · 演示文稿' });
    fireEvent.pointerDown(trigger, { button: 0, ctrlKey: false });
    fireEvent.click(trigger);
    expect(screen.getByRole('radiogroup', { name: '页面范围' })).toBeInTheDocument();
    expect(screen.getByRole('radiogroup', { name: '修改对象' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('radio', { name: '页面范围：自选页' }));
    expect(base.onSelectionChange).toHaveBeenCalledWith('custom_pages');
  });

  it('shows page titles in the stacked custom window without a count in the trigger', () => {
    render(<TargetSelector {...base} selection="custom_pages" selectedSlideIds={['sli_2']} />);
    const trigger = screen.getByRole('button', { name: '范围：自选页 · 演示文稿' });
    expect(trigger).toHaveTextContent('自选页 · 演示文稿');
    expect(trigger).not.toHaveTextContent('2 页');
    fireEvent.pointerDown(trigger, { button: 0, ctrlKey: false });
    fireEvent.click(trigger);
    expect(screen.getByText('封面')).toBeInTheDocument();
    expect(screen.getByText('结论')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('option', { name: /封面/ }));
    expect(base.onToggleSlide).toHaveBeenCalledWith('sli_1');
  });

  it('locks global resources to all pages', () => {
    render(<TargetSelector {...base} object="global" selection="all_pages" />);
    const trigger = screen.getByRole('button', { name: '范围：全部页 · 全局资源' });
    fireEvent.pointerDown(trigger, { button: 0, ctrlKey: false });
    fireEvent.click(trigger);
    const customSection = screen.getByRole('radio', { name: '页面范围：自选章' });
    expect(customSection).toHaveAttribute('aria-disabled', 'true');
    expect(customSection).toHaveAttribute('data-disabled');
  });
});
