import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogTrigger,
} from './dialog';

describe('overlay Escape dismissal', () => {
  it('closes dialogs without restoring focus to the trigger', async () => {
    render(
      <Dialog>
        <DialogTrigger asChild>
          <button type="button">打开弹窗</button>
        </DialogTrigger>
        <DialogContent>
          <DialogTitle>测试弹窗</DialogTitle>
        </DialogContent>
      </Dialog>,
    );

    const trigger = screen.getByRole('button', { name: '打开弹窗' });
    trigger.focus();
    fireEvent.click(trigger);
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' });

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(trigger).not.toHaveFocus();
  });
});
