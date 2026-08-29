import { act, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useToastStore } from '../../stores/toastStore';
import { GlobalErrorToasts } from './GlobalErrorToasts';

describe('GlobalErrorToasts', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    useToastStore.getState().clearErrors();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('stacks errors in appearance order', () => {
    render(<GlobalErrorToasts />);
    act(() => {
      useToastStore.getState().pushError('第一个错误');
      useToastStore.getState().pushError('第二个错误');
    });

    expect(screen.getAllByRole('alert').map((toast) => toast.textContent)).toEqual([
      '第一个错误',
      '第二个错误',
    ]);
  });

  it('dismisses an error five seconds after it appears', () => {
    render(<GlobalErrorToasts />);
    act(() => {
      useToastStore.getState().pushError('自动关闭');
    });

    act(() => {
      vi.advanceTimersByTime(5_000);
    });
    expect(screen.getByRole('alert')).toHaveClass('global-error-toast-leaving');

    act(() => {
      vi.advanceTimersByTime(220);
    });
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('supports manual dismissal', () => {
    render(<GlobalErrorToasts />);
    act(() => {
      useToastStore.getState().pushError('手动关闭');
    });

    fireEvent.click(screen.getByRole('button', { name: '关闭错误通知' }));
    act(() => {
      vi.advanceTimersByTime(220);
    });

    expect(screen.queryByText('手动关闭')).not.toBeInTheDocument();
  });
});
