import { act, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { showGlobalWarning, useToastStore } from '../../stores/toastStore';
import { GlobalToasts } from './GlobalToasts';

describe('GlobalToasts', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    useToastStore.getState().clearToasts();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('renders reusable semantic tones in appearance order', () => {
    render(<GlobalToasts />);
    act(() => {
      const { pushToast } = useToastStore.getState();
      pushToast('普通通知');
      pushToast('操作成功', 'success');
      showGlobalWarning('需要注意');
      pushToast('操作失败', 'error');
    });

    expect(screen.getAllByRole('status').map((toast) => toast.textContent)).toEqual([
      '普通通知',
      '操作成功',
      '需要注意',
    ]);
    expect(screen.getByRole('alert')).toHaveTextContent('操作失败');
    expect(screen.getByText('普通通知').closest('article')).toHaveClass('global-toast-neutral');
    expect(screen.getByText('操作成功').closest('article')).toHaveClass('global-toast-success');
    expect(screen.getByText('需要注意').closest('article')).toHaveClass('global-toast-warning');
    expect(screen.getByText('操作失败').closest('article')).toHaveClass('global-toast-error');
  });

  it('dismisses a Toast five seconds after it appears', () => {
    render(<GlobalToasts />);
    act(() => {
      useToastStore.getState().pushToast('自动关闭', 'error');
    });

    act(() => {
      vi.advanceTimersByTime(5_000);
    });
    expect(screen.getByRole('alert')).toHaveClass('global-toast-leaving');

    act(() => {
      vi.advanceTimersByTime(220);
    });
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('supports manual dismissal', () => {
    render(<GlobalToasts />);
    act(() => {
      useToastStore.getState().pushToast('手动关闭', 'warning');
    });

    fireEvent.click(screen.getByRole('button', { name: '关闭通知' }));
    act(() => {
      vi.advanceTimersByTime(220);
    });

    expect(screen.queryByText('手动关闭')).not.toBeInTheDocument();
  });
});
