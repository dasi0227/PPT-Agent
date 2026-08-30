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

describe('ProjectTabs warehouse menu', () => {
  beforeEach(() => {
    useProjectStore.setState({
      projects: [],
      openProjectIds: [],
      activeProjectId: null,
      loadingProjects: false,
      loadProjects: vi.fn().mockResolvedValue(undefined),
    });
  });

  it('opens the compact menu and navigates to all repository sections', async () => {
    const user = userEvent.setup();
    render(
      <MemoryRouter>
        <ProjectTabs />
        <LocationProbe />
      </MemoryRouter>,
    );

    await user.click(screen.getByRole('button', { name: '个人仓库' }));
    expect(screen.getByText('选择演示文稿的整体样式')).toBeInTheDocument();
    expect(screen.getByText('浏览供 Agent 参考的片段')).toBeInTheDocument();
    expect(screen.getByText('管理 Run 可使用的技能')).toBeInTheDocument();

    await user.click(screen.getByText('组件'));
    expect(screen.getByLabelText('location')).toHaveTextContent('/warehouse/component');
  });
});
