import { fireEvent, render, screen } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { DOMSelectionReference } from './DOMSelectionReference';
import type { DOMSelection } from '../../api/types';
afterEach(() => vi.unstubAllGlobals());
it('finishes comments with the button only, never Command/Ctrl Enter', () => {
  vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} });
  const editing=vi.fn();
  render(<DOMSelectionReference selection={{marker_no:1,comment:'调整标题'} as DOMSelection} editing onEditingChange={editing} onCommentChange={vi.fn()} onNavigate={vi.fn()} onRemove={vi.fn()} onFocusComposer={vi.fn()} />);
  const input=screen.getByRole('textbox',{name:'标记 1 注释'});
  fireEvent.keyDown(input,{key:'Enter',metaKey:true});
  fireEvent.keyDown(input,{key:'Enter',ctrlKey:true});
  expect(editing).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole('button',{name:'完成'}));
  expect(editing).toHaveBeenCalledWith(false);
});
