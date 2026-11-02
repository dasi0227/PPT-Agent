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

  it('supports manual dismissal and consumes a Toast action once', () => {
    render(<GlobalToasts />);
    act(() => {
      useToastStore.getState().pushToast('手动关闭', 'warning');
    });

    fireEvent.click(screen.getByRole('button', { name: '关闭通知' }));
    act(() => {
      vi.advanceTimersByTime(220);
    });

    expect(screen.queryByText('手动关闭')).not.toBeInTheDocument();

    const undo = vi.fn();
    act(() => {
      useToastStore.getState().pushToast('已删除条目', 'success', { label: '撤销', onClick: undo });
    });
    const action = screen.getByRole('button', { name: '撤销' });
    fireEvent.click(action);
    fireEvent.click(action);
    expect(undo).toHaveBeenCalledTimes(1);
    act(() => { vi.advanceTimersByTime(220); });
    expect(screen.queryByText('已删除条目')).not.toBeInTheDocument();
  });

  it('uses the remaining undo deadline for dismissal and countdown', () => {
    render(<GlobalToasts />);
    const undo = vi.fn();
    act(() => {
      useToastStore.getState().pushToast('项目已删除', 'success', { label: '撤销', onClick: undo }, { expiresAt: Date.now() + 5_000 });
    });
    expect(document.querySelector('.global-toast-timer')).toHaveStyle({ animationDuration: '5000ms' });
    act(() => { vi.advanceTimersByTime(3_000); });
    expect(screen.getByRole('button', { name: '撤销' })).toBeEnabled();
    act(() => { vi.advanceTimersByTime(2_000); });
    fireEvent.click(screen.getByRole('button', { name: '撤销' }));
    expect(undo).not.toHaveBeenCalled();
    expect(screen.getByRole('status')).toHaveClass('global-toast-leaving');
  });
});
