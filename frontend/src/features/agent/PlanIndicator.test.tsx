import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import type { PlanState } from '../../api/types';
import { PlanIndicator } from './PlanIndicator';

const plan: PlanState = {
  id: 'plan-1',
  title: '制作产品演示',
  content: '',
  status: 'active',
  steps: [
    { id: 'step-1', title: '确认目标', detail: '不应显示的详情', status: 'completed' },
    { id: 'step-2', title: '确定视觉方向', status: 'completed' },
    { id: 'step-3', title: '编写页面', status: 'in_progress' },
    { id: 'step-4', title: '最终复核', status: 'pending' },
  ],
};

describe('PlanIndicator', () => {
  it('renders plan steps as a connected status timeline with titles only', () => {
    render(<PlanIndicator plan={plan} running />);

    const trigger = screen.getByRole('button', { name: '查看计划进度 2 / 4' });
    expect(trigger).not.toHaveTextContent('2/4');
    fireEvent.pointerDown(trigger, { button: 0, ctrlKey: false });
    fireEvent.click(trigger);

    expect(document.querySelectorAll('[data-plan-step-status="completed"]')).toHaveLength(2);
    expect(document.querySelector('[data-plan-step-status="completed"]')).toHaveClass('border-success/45', 'bg-success-soft', 'text-success');
    expect(document.querySelectorAll('[data-plan-step-status="in_progress"]')).toHaveLength(1);
    expect(document.querySelectorAll('[data-plan-step-status="pending"]')).toHaveLength(1);
    expect(document.querySelectorAll('[data-plan-step-connector="true"]')).toHaveLength(3);
    expect(document.querySelectorAll('[data-plan-step-connector][data-reached="true"]')).toHaveLength(2);
    expect(screen.getByText('编写页面').closest('li')).toHaveClass('bg-accent-soft/80');
    expect(screen.queryByText('不应显示的详情')).not.toBeInTheDocument();
    expect(screen.queryByText('已完成')).not.toBeInTheDocument();
    expect(screen.queryByText('正在执行')).not.toBeInTheDocument();
    expect(screen.queryByText('等待执行')).not.toBeInTheDocument();
    expect(screen.getAllByRole('img', { name: '已完成' })).toHaveLength(2);
    expect(screen.getByRole('img', { name: '正在执行' })).toBeInTheDocument();
    expect(screen.getByRole('img', { name: '等待执行' })).toBeInTheDocument();
  });

  it('animates the newly completed node and reveals the connector to the next active step', () => {
    const { rerender } = render(<PlanIndicator plan={plan} running />);
    const trigger = screen.getByRole('button', { name: '查看计划进度 2 / 4' });
    fireEvent.pointerDown(trigger, { button: 0, ctrlKey: false });
    fireEvent.click(trigger);

    const advancedPlan: PlanState = {
      ...plan,
      steps: plan.steps.map((step, index) => (
        index === 2 ? { ...step, status: 'completed' }
          : index === 3 ? { ...step, status: 'in_progress' }
            : step
      )),
    };
    rerender(<PlanIndicator plan={advancedPlan} running />);

    const completedRow = screen.getByText('编写页面').closest('li');
    expect(completedRow?.querySelector('[data-plan-step-status="completed"]')).toHaveClass('plan-step-node-completed');
    expect(document.querySelectorAll('[data-plan-step-connector][data-reached="true"]')).toHaveLength(3);
    expect(screen.getByText('最终复核').closest('li')).toHaveClass('bg-accent-soft/80');
  });

  it('does not restore focus to the trigger after a pointer dismissal', async () => {
    render(<PlanIndicator plan={plan} running />);
    const trigger = screen.getByRole('button', { name: '查看计划进度 2 / 4' });
    fireEvent.pointerDown(trigger, { button: 0, ctrlKey: false });
    fireEvent.click(trigger);

    expect(screen.getByRole('menu')).toBeInTheDocument();
    await act(() => new Promise((resolve) => window.setTimeout(resolve, 0)));
    fireEvent.pointerDown(document.body, { button: 0 });
    fireEvent.click(document.body);

    await waitFor(() => expect(screen.queryByRole('menu')).not.toBeInTheDocument());
    expect(trigger).not.toHaveFocus();
  });

  it('does not restore focus to the trigger after an Escape-key dismissal', async () => {
    render(<PlanIndicator plan={plan} running />);
    const trigger = screen.getByRole('button', { name: '查看计划进度 2 / 4' });
    fireEvent.keyDown(trigger, { key: 'Enter' });

    const menu = screen.getByRole('menu');
    fireEvent.keyDown(menu, { key: 'Escape' });

    await waitFor(() => expect(screen.queryByRole('menu')).not.toBeInTheDocument());
    expect(trigger).not.toHaveFocus();
  });
});
