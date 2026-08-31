import { fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useProjectStore } from '../../stores/projectStore';
import { OpenExistingProjectModal } from './OpenExistingProjectModal';

describe('OpenExistingProjectModal', () => {
  const onOpenChange = vi.fn();
  const onBack = vi.fn();
  const loadProjects = vi.fn();
  const openProject = vi.fn();

  beforeEach(() => {
    onOpenChange.mockReset();
    onBack.mockReset();
    loadProjects.mockReset();
    openProject.mockReset();
    useProjectStore.setState({ projects: [], loadProjects, openProject });
  });

  it('returns to the project action picker when there are no existing projects', () => {
    render(
      <MemoryRouter>
        <OpenExistingProjectModal open onOpenChange={onOpenChange} onBack={onBack} />
      </MemoryRouter>,
    );

    fireEvent.click(screen.getByRole('button', { name: '返回项目操作' }));
    expect(onBack).toHaveBeenCalledTimes(1);
    expect(onOpenChange).not.toHaveBeenCalledWith(false);
  });

  it('keeps the back action available when existing projects are listed', () => {
    useProjectStore.setState({
      projects: [{
        id: 'project-1',
        title: '已有项目',
        work_dir: '',
        theme: 'default',
        status: 'ready',
        design_path: '',
        created_at: 1,
        updated_at: 1,
      }],
    });

    render(
      <MemoryRouter>
        <OpenExistingProjectModal open onOpenChange={onOpenChange} onBack={onBack} />
      </MemoryRouter>,
    );

    fireEvent.click(screen.getByRole('button', { name: '返回项目操作' }));
    expect(onBack).toHaveBeenCalledTimes(1);
  });

  it('opens a project only after the selected project is confirmed', () => {
    useProjectStore.setState({
      projects: [{
        id: 'project-1',
        title: '已有项目',
        work_dir: '',
        theme: 'default',
        status: 'ready',
        design_path: '',
        created_at: 1,
        updated_at: 1,
      }],
    });

    render(
      <MemoryRouter>
        <OpenExistingProjectModal open onOpenChange={onOpenChange} onBack={onBack} />
      </MemoryRouter>,
    );

    const confirm = screen.getByRole('button', { name: '打开项目' });
    expect(confirm).toBeDisabled();

    fireEvent.click(screen.getByRole('button', { name: /已有项目/ }));
    expect(openProject).not.toHaveBeenCalled();
    expect(onOpenChange).not.toHaveBeenCalledWith(false);
    expect(confirm).toBeEnabled();

    fireEvent.click(confirm);
    expect(openProject).toHaveBeenCalledWith('project-1');
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });
});
