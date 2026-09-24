import { fireEvent, render, screen } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { ExportButton } from './ExportButton';
import { useShortcutStore } from '../../stores/shortcutStore';
import { defaultBindings } from '../../lib/shortcuts';
import { isMac } from '../../lib/platform';
afterEach(()=>useShortcutStore.setState({ready:false,bindings:defaultBindings}));
it('opens a central dialog from the shortcut and exports only the selected format', () => {
  useShortcutStore.setState({ready:true,bindings:defaultBindings});
  const onExport=vi.fn();render(<ExportButton disabled={false} onExport={onExport} />);
  fireEvent.keyDown(window,{code:'KeyP',...(isMac()?{metaKey:true}:{ctrlKey:true})});
  expect(screen.getByRole('dialog',{name:'导出演示文稿'})).toBeInTheDocument();
  expect(onExport).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole('button',{name:'PDF 文档'}));
  expect(onExport).toHaveBeenCalledWith('pdf');
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
});
it('keeps both click and shortcut disabled without slides', () => {
  useShortcutStore.setState({ready:true});
  render(<ExportButton disabled reason="暂无幻灯片可导出" onExport={vi.fn()} />);
  expect(screen.getByRole('button',{name:'导出'})).toBeDisabled();
  fireEvent.keyDown(window,{code:'KeyP',...(isMac()?{metaKey:true}:{ctrlKey:true})});
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
});
