import { act, fireEvent, render } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { useAppShortcuts } from './useAppShortcuts';
import { useShortcutStore } from '../stores/shortcutStore';
import { defaultBindings } from './shortcuts';
import { isMac } from './platform';
function Probe({ run }: { run: () => void }) { useAppShortcuts({ 'deck.overview': run }); return <input aria-label="编辑" />; }
afterEach(() => useShortcutStore.setState({ bindings: defaultBindings, ready:false }));
it('updates live and protects editing, overlays, and held keys', () => {
  useShortcutStore.setState({ bindings:defaultBindings, ready:true });
  const run=vi.fn(); const {getByLabelText}=render(<Probe run={run} />);
  const modifiers = isMac() ? {metaKey:true} : {ctrlKey:true};
  fireEvent.keyDown(window,{code:'KeyO',...modifiers}); expect(run).toHaveBeenCalledTimes(1);
  fireEvent.keyDown(getByLabelText('编辑'),{code:'KeyO',...modifiers});
  fireEvent.keyDown(window,{code:'KeyO',repeat:true,...modifiers}); expect(run).toHaveBeenCalledTimes(1);
  const dialog=document.createElement('div');dialog.setAttribute('role','dialog');dialog.dataset.state='open';document.body.append(dialog);
  fireEvent.keyDown(window,{code:'KeyO',...modifiers}); expect(run).toHaveBeenCalledTimes(1);dialog.remove();
  act(()=>useShortcutStore.setState({bindings:{...defaultBindings,'deck.overview':{code:'KeyB',primary:true}}}));
  fireEvent.keyDown(window,{code:'KeyO',...modifiers}); expect(run).toHaveBeenCalledTimes(1);
  fireEvent.keyDown(window,{code:'KeyB',...modifiers}); expect(run).toHaveBeenCalledTimes(2);
});
