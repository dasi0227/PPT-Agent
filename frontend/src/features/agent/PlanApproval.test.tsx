import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { runsApi } from '../../api/runs';
import type { PlanApprovalItem } from './eventReducer';
import { PlanApproval } from './PlanApproval';

vi.mock('../../api/runs', () => ({
  runsApi: { submitPlanApproval: vi.fn() },
}));

const item: PlanApprovalItem = {
  id: 'run-1:plan-approval:interaction-1',
  type: 'plan_approval',
  runId: 'run-1',
  interactionId: 'interaction-1',
  timestamp: 0,
  plan: {
    id: 'plan-1',
    revision: 3,
    status: 'awaiting_approval',
    title: '演示文稿制作计划',
    content: '先完成结构，再生成页面。',
    steps: [{ id: 'step-1', title: '完成结构', status: 'pending' }],
  },
};

describe('PlanApproval', () => {
  it('keeps the approval card visible and changes only the selected decision', () => {
    render(<PlanApproval item={item} />);

    const approve = screen.getByRole('button', { name: '批准执行' });
    expect(screen.getByText('演示文稿制作计划')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '展开全部' })).toBeNull();
    expect(approve).toHaveAttribute('aria-pressed', 'false');
    expect(screen.getByRole('button', { name: '提交' })).toBeDisabled();

    fireEvent.click(approve);

    expect(screen.getByText('演示文稿制作计划')).toBeInTheDocument();
    expect(approve).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByRole('button', { name: '提交' })).not.toBeDisabled();
    expect(runsApi.submitPlanApproval).not.toHaveBeenCalled();
  });

  it('shows an expandable fade only when pending plan content overflows', () => {
    const scrollHeight = vi.spyOn(HTMLElement.prototype, 'scrollHeight', 'get').mockImplementation(function (this: HTMLElement) {
      return this.dataset.testid === 'plan-content-preview' ? 220 : 0;
    });
    const clientHeight = vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockImplementation(function (this: HTMLElement) {
      return this.dataset.testid === 'plan-content-preview' ? 96 : 0;
    });

    try {
      render(<PlanApproval item={{
        ...item,
        plan: {
          ...item.plan,
          content: '先完成结构。\n\n再生成全部页面。\n\n逐页检查视觉和布局。\n\n最后统一复核。',
        },
      }} />);

      const approve = screen.getByRole('button', { name: '批准执行' });
      fireEvent.click(approve);
      expect(approve).toHaveAttribute('aria-pressed', 'true');
      expect(screen.getByTestId('plan-content-preview')).toHaveClass('max-h-[480px]', 'overflow-hidden');

      fireEvent.click(screen.getByRole('button', { name: '展开全部' }));
      expect(screen.getByTestId('plan-content-preview')).not.toHaveClass('max-h-[480px]');
      expect(screen.getByRole('button', { name: '收起' })).toBeInTheDocument();
      expect(approve).toHaveAttribute('aria-pressed', 'true');

      fireEvent.click(screen.getByRole('button', { name: '收起' }));
      expect(screen.getByRole('button', { name: '展开全部' })).toBeInTheDocument();
    } finally {
      scrollHeight.mockRestore();
      clientHeight.mockRestore();
    }
  });

  it('separates the decision controls and requires feedback for a revision', () => {
    render(<PlanApproval item={item} />);

    expect(screen.getByTestId('plan-approval-actions')).toHaveClass('border-t');
    fireEvent.click(screen.getByRole('button', { name: '返回修改' }));
    expect(screen.getByPlaceholderText('说明需要调整的内容')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '提交' })).toBeDisabled();

    fireEvent.change(screen.getByPlaceholderText('说明需要调整的内容'), { target: { value: '需要调整标题' } });
    expect(screen.getByRole('button', { name: '提交' })).not.toBeDisabled();
  });

  it('folds an answered approval into a plan row and keeps the decision read-only', () => {
    render(<PlanApproval item={{ ...item, answer: { decision: 'revise', feedback: '请补充案例' } }} />);

    const toggle = screen.getByRole('button', { name: '计划已返回修改' });
    expect(screen.queryByRole('button', { name: '提交' })).toBeNull();
    expect(screen.queryByText('请补充案例')).toBeNull();

    fireEvent.click(toggle);

    expect(screen.getByText('请补充案例')).toBeInTheDocument();
    expect(screen.getByText('演示文稿制作计划')).toBeInTheDocument();
    expect(screen.getByText('返回修改')).toHaveAttribute('aria-pressed', 'true');
    expect(screen.queryByRole('button', { name: '提交' })).toBeNull();
  });
});
