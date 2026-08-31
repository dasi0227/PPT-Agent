import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useProjectStore } from '../../stores/projectStore';
import { ProjectPickerModal } from './ProjectPickerModal';

describe('ProjectPickerModal', () => {
  const onOpenChange = vi.fn();
  const onOpenExisting = vi.fn();
  const createProject = vi.fn();
  const openProject = vi.fn();

  beforeEach(() => {
    onOpenChange.mockReset();
    onOpenExisting.mockReset();
    createProject.mockReset();
    openProject.mockReset();
    useProjectStore.setState({ createProject, openProject });
  });

  it('creates a real project only after the user confirms its name', async () => {
    createProject.mockResolvedValue({ id: 'project-1' });
    render(
      <MemoryRouter>
        <ProjectPickerModal open onOpenChange={onOpenChange} onOpenExisting={onOpenExisting} />
      </MemoryRouter>,
    );

    expect(screen.queryByText('选择操作')).not.toBeInTheDocument();
    expect(screen.queryByText('新建一个演示文稿，或继续已有项目。')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '创建全新项目' }));
    expect(screen.getByRole('heading', { name: '创建全新项目' })).toBeInTheDocument();
    expect(screen.queryByText('为这个演示文稿输入一个便于识别的名称。')).not.toBeInTheDocument();
    expect(screen.queryByText('创建后可随时在项目菜单中重命名。')).not.toBeInTheDocument();
    expect(createProject).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText('项目名称'), { target: { value: '产品发布会' } });
    fireEvent.click(screen.getByRole('button', { name: '创建项目' }));

    await waitFor(() => expect(createProject).toHaveBeenCalledWith('产品发布会', '', 10, 'zh-CN'));
    expect(openProject).toHaveBeenCalledWith('project-1');
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it('returns from project creation through the title action', () => {
    render(
      <MemoryRouter>
        <ProjectPickerModal open onOpenChange={onOpenChange} onOpenExisting={onOpenExisting} />
      </MemoryRouter>,
    );

    fireEvent.click(screen.getByRole('button', { name: '创建全新项目' }));
    fireEvent.click(screen.getByRole('button', { name: '返回项目操作' }));

    expect(screen.getByRole('button', { name: '创建全新项目' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '打开已有项目' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '返回' })).not.toBeInTheDocument();
  });
});
