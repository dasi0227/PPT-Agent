import { act, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, expect, it } from 'vitest';
import { useProjectStore } from '../../stores/projectStore';
import { useThreadStore } from '../../stores/threadStore';
import { useSessionStore } from '../../stores/sessionStore';
import { sessionFixture } from '../../test/sessionFixtures';
import { Timeline } from './Timeline';

beforeEach(() => {
  useProjectStore.setState({ activeProjectId: 'p1' });
  useThreadStore.setState({ activeThreadIdByProjectId: { p1: 't1' } });
  useSessionStore.setState({ sessions: {} });
});
afterEach(() => useSessionStore.getState().drop(['t1']));

it('shows init only after an empty view succeeds, never on loading or error', () => {
  render(<Timeline />);
  expect(screen.getByLabelText('加载中')).toBeInTheDocument();
  expect(screen.queryByRole('button', { name: 'Dasi PPT Agent' })).not.toBeInTheDocument();
  act(() => useSessionStore.setState({ sessions: { t1: sessionFixture({ loadStatus: 'error', isEmpty: false, error: new Error('offline') }) } }));
  expect(screen.getByRole('alert')).toHaveTextContent('offline');
  expect(screen.queryByRole('button', { name: 'Dasi PPT Agent' })).not.toBeInTheDocument();
  act(() => useSessionStore.setState({ sessions: { t1: sessionFixture() } }));
  expect(screen.getByRole('button', { name: 'Dasi PPT Agent' })).toBeInTheDocument();
});
