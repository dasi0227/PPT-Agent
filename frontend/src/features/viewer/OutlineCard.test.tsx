import { render, screen, fireEvent } from '@testing-library/react';
import { describe, expect, test, vi } from 'vitest';
import { OutlineCard } from './OutlineCard';
import { Slide } from '../../api/types';

const baseSlide: Slide = {
  id: 's1', project_id: 'p1', order: 10, outline_dirty: true, layout: 'bullets', title: '市场分析',
  html_path: '', json_path: '', current_version: 0, idx: 0,
  content: { layout: 'bullets', title: '市场分析', subtitle: '2026 展望', bullets: ['增长放缓', '头部集中'] },
};

describe('OutlineCard', () => {
  test('renders title bullets and dirty badge', () => {
    render(<OutlineCard slide={baseSlide} editable={false} onPatch={() => {}} dirty />);
    expect(screen.getByText('市场分析')).toBeInTheDocument();
    expect(screen.getByText('增长放缓')).toBeInTheDocument();
    expect(screen.getByText('头部集中')).toBeInTheDocument();
    expect(screen.getByText(/待更新/)).toBeInTheDocument();
  });

  test('non-editable renders no inputs', () => {
    render(<OutlineCard slide={baseSlide} editable={false} onPatch={() => {}} dirty={false} />);
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
  });

  test('editable title blur triggers onPatch with changed title', () => {
    const onPatch = vi.fn();
    render(<OutlineCard slide={baseSlide} editable onPatch={onPatch} dirty={false} />);
    const titleInput = screen.getByDisplayValue('市场分析');
    fireEvent.change(titleInput, { target: { value: '新市场分析' } });
    fireEvent.blur(titleInput);
    expect(onPatch).toHaveBeenCalledWith({ title: '新市场分析' });
  });

  test('editable title blur without change does not call onPatch', () => {
    const onPatch = vi.fn();
    render(<OutlineCard slide={baseSlide} editable onPatch={onPatch} dirty={false} />);
    const titleInput = screen.getByDisplayValue('市场分析');
    fireEvent.blur(titleInput);
    expect(onPatch).not.toHaveBeenCalled();
  });
});
