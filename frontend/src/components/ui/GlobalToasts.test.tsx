import { act, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useToastStore } from '../../stores/toastStore';
import { GlobalToasts } from './GlobalToasts';

describe('GlobalToasts', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    useToastStore.getState().clearToasts();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('dismisses a Toast three seconds after it appears', () => {
    render(<GlobalToasts />);
    act(() => {
      useToastStore.getState().pushToast('自动关闭', 'error');
    });

    act(() => {
      vi.advanceTimersByTime(3_000);
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
