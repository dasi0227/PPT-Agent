import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { ContextWindowPanel } from './ContextWindowPanel';

describe('ContextWindowPanel', () => {
  it('closes with Escape without retaining focus on its trigger', () => {
    render(<ContextWindowPanel />);
    const trigger = screen.getByRole('button', { name: /上下文窗口/ });
    trigger.focus();
    fireEvent.click(trigger);

    expect(screen.getByRole('dialog', { name: '上下文窗口' })).toBeInTheDocument();
    fireEvent.keyDown(document, { key: 'Escape' });

    expect(screen.queryByRole('dialog', { name: '上下文窗口' })).not.toBeInTheDocument();
    expect(trigger).not.toHaveFocus();
  });

  it('shows all six buckets in the fixed order, including zero values', () => {
    render(<ContextWindowPanel />);
    fireEvent.click(screen.getByRole('button', { name: /上下文窗口/ }));

    expect(screen.getAllByRole('tab').map((tab) => tab.textContent)).toEqual([
      '系统提示词0.0 k',
      '运行时0.0 k',
      '对话历史0.0 k',
      '读文件0.0 k',
      '跑命令0.0 k',
      '其它0.0 k',
    ]);
    expect(screen.queryByText('空闲')).not.toBeInTheDocument();
    expect(screen.getByText('system prompts')).toBeInTheDocument();
    expect(screen.getByText('定义 Agent 行为、模式与任务约束')).toBeInTheDocument();
    expect(screen.getAllByText('0.00 k')).toHaveLength(2);
  });

  it('keeps fixed zero-token details visible when switching buckets', () => {
    render(<ContextWindowPanel />);
    fireEvent.click(screen.getByRole('button', { name: /上下文窗口/ }));
    fireEvent.click(screen.getByRole('tab', { name: /读文件/ }));

    expect(screen.getByRole('tabpanel', { name: '读文件明细' })).toBeInTheDocument();
    expect(screen.getByText('read_ppt')).toBeInTheDocument();
    expect(screen.getByText('read_image')).toBeInTheDocument();
    expect(screen.getByText('read_project')).toBeInTheDocument();
    expect(screen.getByText('读取或上传并送入模型的图片')).toBeInTheDocument();
    expect(screen.getAllByText('0.00 k')).toHaveLength(3);
  });
});
