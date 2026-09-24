import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { repositoriesApi } from '../../api/repositories';
import type { Project, Theme } from '../../api/types';
import { useProjectStore } from '../../stores/projectStore';
import { useShortcutStore } from '../../stores/shortcutStore';
import { defaultBindings } from '../../lib/shortcuts';
import { isMac } from '../../lib/platform';
import { ThemeSelector } from './ThemeSelector';

const initialProjectState = useProjectStore.getState();
const themes = [{id:'a',name:'Alpha'}, {id:'b',name:'Beta'}, {id:'c',name:'Gamma'}] as Theme[];
const press = () => fireEvent.keyDown(window, {code:'KeyT', ...(isMac() ? {metaKey:true} : {ctrlKey:true})});
afterEach(() => {
  vi.restoreAllMocks();
  act(() => {
    useProjectStore.setState(initialProjectState);
    useShortcutStore.setState({ready:false,bindings:defaultBindings});
  });
});
it('applies the next theme, wraps to the first, and ignores concurrent requests without opening a menu', async () => {
  vi.spyOn(repositoriesApi, 'listThemes').mockResolvedValue({themes});
  let finish: () => void = () => {};
  const apply = vi.fn((projectId: string, themeId: string) => new Promise<Project>(resolve => {
    finish = () => {
      const project = {id:projectId,theme:themeId} as Project;
      useProjectStore.setState({projects:[project]});
      resolve(project);
    };
  }));
  useProjectStore.setState({projects:[{id:'p1',theme:'b'} as Project],contentByProjectId:{},setProjectTheme:apply});
  useShortcutStore.setState({ready:true,bindings:defaultBindings});
  render(<ThemeSelector projectId="p1" />);
  await screen.findByText('Beta');
  press(); press();
  expect(apply).toHaveBeenCalledTimes(1);
  expect(apply).toHaveBeenLastCalledWith('p1','c');
  expect(screen.queryByRole('menu')).not.toBeInTheDocument();
  await act(async () => finish());
  expect(screen.getByText('Gamma')).toBeInTheDocument();
  press();
  expect(apply).toHaveBeenLastCalledWith('p1','a');
  await act(async () => finish());
  expect(screen.getByText('Alpha')).toBeInTheDocument();
});
it('waits for the initial theme list before cycling', async () => {
  let finishLoad: (value: {themes: Theme[]}) => void = () => {};
  const load = vi.spyOn(repositoriesApi,'listThemes').mockImplementation(() => new Promise(resolve => {finishLoad=resolve;}));
  const apply = vi.fn().mockResolvedValue({id:'p1',theme:'b'});
  useProjectStore.setState({projects:[{id:'p1',theme:'a'} as Project],contentByProjectId:{},setProjectTheme:apply});
  useShortcutStore.setState({ready:true,bindings:defaultBindings});
  render(<ThemeSelector projectId="p1" />);
  press();
  expect(apply).not.toHaveBeenCalled();
  await act(async () => finishLoad({themes}));
  await waitFor(() => expect(apply).toHaveBeenCalledWith('p1','b'));
  expect(load).toHaveBeenCalledTimes(1);
  expect(screen.queryByRole('menu')).not.toBeInTheDocument();
});
