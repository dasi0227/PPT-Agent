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
});
