import { fireEvent, render, screen, waitFor } from '@testing-library/react';
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
    render(<ProjectPickerModal open onOpenChange={onOpenChange} onOpenExisting={onOpenExisting} />);

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
});
