import { fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useProjectStore } from '../../stores/projectStore';
import { OpenExistingProjectModal } from './OpenExistingProjectModal';

describe('OpenExistingProjectModal', () => {
  const onOpenChange = vi.fn();
  const onBack = vi.fn();
  const loadProjects = vi.fn();

  beforeEach(() => {
    onOpenChange.mockReset();
    onBack.mockReset();
    loadProjects.mockReset();
    useProjectStore.setState({ projects: [], loadProjects });
  });

  it('returns to the project action picker when there are no existing projects', () => {
    render(
      <MemoryRouter>
        <OpenExistingProjectModal open onOpenChange={onOpenChange} onBack={onBack} />
      </MemoryRouter>,
    );

    fireEvent.click(screen.getByRole('button', { name: '返回' }));
    expect(onBack).toHaveBeenCalledTimes(1);
    expect(onOpenChange).not.toHaveBeenCalledWith(false);
  });
});
