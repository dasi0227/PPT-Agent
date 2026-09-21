import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, useLocation } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useProjectStore } from '../../stores/projectStore';
import { ProjectTabs } from './ProjectTabs';

vi.mock('../agent/useActiveSession', () => ({
  useActiveSession: () => ({ status: 'idle' }),
}));

vi.mock('./ProjectMenu', () => ({
  ProjectMenu: ({ children }: { children: React.ReactNode }) => children,
}));

function LocationProbe() {
  return <output aria-label="location">{useLocation().pathname}</output>;
}

describe('ProjectTabs warehouse entry', () => {
  beforeEach(() => {
    useProjectStore.setState({
      projects: [],
      openProjectIds: [],
      activeProjectId: null,
      loadingProjects: false,
      loadProjects: vi.fn().mockResolvedValue(undefined),
    });
  });

  it('navigates directly to the warehouse and preserves the project route', async () => {
    const user = userEvent.setup();
    render(
      <MemoryRouter initialEntries={['/projects/project-7?slide=slide-2']}>
        <ProjectTabs />
        <LocationProbe />
      </MemoryRouter>,
    );

    const repositoryButton = screen.getByRole('button', { name: '仓库' });
    expect(repositoryButton).toHaveTextContent('仓库');
    await user.click(repositoryButton);
    expect(screen.getByLabelText('location')).toHaveTextContent('/warehouse/theme');
    expect(screen.queryByText('选择演示文稿的整体样式')).not.toBeInTheDocument();
  });
});
