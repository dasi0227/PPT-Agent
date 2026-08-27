import { act, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Project } from '../../api/types';
import { useProjectStore } from '../../stores/projectStore';
import { useThreadStore } from '../../stores/threadStore';
import { WorkspaceRoute } from './WorkspaceRoute';

vi.mock('./AppShell', () => ({ AppShell: () => <div data-testid="app-shell" /> }));
vi.mock('./useWorkspaceUrlState', () => ({ useWorkspaceUrlState: vi.fn() }));

function LocationProbe() {
  const location = useLocation();
  return <div data-testid="location">{location.pathname}</div>;
}

function renderWorkspaceRoute(initialPath: string) {
  render(
    <MemoryRouter initialEntries={[initialPath]}>
      <LocationProbe />
      <Routes>
        <Route path="/" element={<div data-testid="home" />} />
        <Route path="/projects/:projectId" element={<WorkspaceRoute />} />
      </Routes>
    </MemoryRouter>,
  );
}

function project(id: string, title = id): Project {
  return {
    id,
    title,
    work_dir: `/tmp/${id}`,
    theme: 'default',
    status: 'ready',
    design_path: '',
    created_at: 0,
    updated_at: 0,
  };
}

describe('WorkspaceRoute project close guard', () => {
  beforeEach(() => {
    useProjectStore.setState({
      projects: [project('pro_1', '项目 1')],
      openProjectIds: [],
      activeProjectId: null,
      contentByProjectId: {},
      contentLoadingByProjectId: {},
      contentErrorByProjectId: {},
      mutationPendingByProjectId: {},
      loadingProjects: false,
      projectError: null,
      loadProjects: vi.fn(async () => undefined),
      loadProjectContent: vi.fn(async () => undefined),
    });
    useThreadStore.setState({
      threadsByProjectId: {},
      openThreadIdsByProjectId: {},
      activeThreadIdByProjectId: {},
      errorByProjectId: {},
      loadThreads: vi.fn(async () => undefined),
    });
  });

  it('navigates home when the current route project is closed and no project remains open', async () => {
    renderWorkspaceRoute('/projects/pro_1');
    await waitFor(() => expect(useProjectStore.getState().activeProjectId).toBe('pro_1'));

    act(() => {
      useProjectStore.getState().closeProject('pro_1');
    });

    await waitFor(() => expect(screen.getByTestId('location')).toHaveTextContent('/'));
    expect(screen.getByTestId('home')).toBeInTheDocument();
  });

  it('navigates to the next open project when the current route project is closed', async () => {
    useProjectStore.setState({
      projects: [
        project('pro_1', '项目 1'),
        project('pro_2', '项目 2'),
      ],
      openProjectIds: ['pro_2'],
      activeProjectId: 'pro_2',
    });
    renderWorkspaceRoute('/projects/pro_1');
    await waitFor(() => expect(useProjectStore.getState().openProjectIds).toContain('pro_1'));

    act(() => {
      useProjectStore.getState().closeProject('pro_1');
    });

    await waitFor(() => expect(screen.getByTestId('location')).toHaveTextContent('/projects/pro_2'));
  });
});
