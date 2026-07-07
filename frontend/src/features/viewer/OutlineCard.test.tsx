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

  test('compact + empty content shows Slide N + 未编辑大纲 + layout badge', () => {
    const empty: Slide = {
      id: 's-empty', project_id: 'p1', order: 2, outline_dirty: false,
      layout: 'bullets', title: '', html_path: '', json_path: '', current_version: 0, idx: 2,
      // 无 content：所有字段缺省。
    };
    render(<OutlineCard slide={empty} editable={false} onPatch={() => {}} dirty={false} compact />);
    expect(screen.getByText('Slide 3')).toBeInTheDocument();
    expect(screen.getByText('未编辑大纲')).toBeInTheDocument();
    // layout 徽标应显示 slide.layout
    expect(screen.getByText('bullets')).toBeInTheDocument();
    // 不应出现输入框（空态是纯占位）
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
  });

  test('!compact + empty content shows 未命名 + 引导语', () => {
    const empty: Slide = {
      id: 's-empty', project_id: 'p1', order: 0, outline_dirty: false,
      layout: 'title', title: '', html_path: '', json_path: '', current_version: 0, idx: 0,
    };
    render(<OutlineCard slide={empty} editable={false} onPatch={() => {}} dirty={false} />);
    expect(screen.getByText('未命名')).toBeInTheDocument();
    expect(screen.getByText('使用右侧对话或点击标题开始编辑')).toBeInTheDocument();
  });
});
