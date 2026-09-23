import { act, render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom';
import { beforeEach, describe, expect, it } from 'vitest';
import type { ProjectContentSnapshot } from '../../api/types';
import { useDeckStore } from '../../stores/deckStore';
import { useProjectStore } from '../../stores/projectStore';
import { useWorkspaceUrlState } from './useWorkspaceUrlState';

function content(ids: string[]): ProjectContentSnapshot {
  return {
    outline: {
      project_id: 'p', sections: [{ id: 'section', title: '章节', slides: ids.map((slide_id) => ({ slide_id, title: slide_id, role: 'content' })), subsections: [] }],
    },
    slides_by_id: {},
  } as unknown as ProjectContentSnapshot;
}

function Probe() {
  useWorkspaceUrlState('p');
  const slideId = useDeckStore((state) => state.currentSlideId);
  const location = useLocation();
  return <output data-testid="selection">{slideId}|{location.search}</output>;
}

describe('workspace URL state during background content updates', () => {
  beforeEach(() => {
    useDeckStore.setState({ currentSlideId: null, globalView: 'html', previewMode: 'main' });
    useProjectStore.setState({ contentByProjectId: { p: content(['s1', 's2', 's3']) } });
  });

  it('keeps the current page and selects its neighbor only when it is deleted', () => {
    render(<MemoryRouter initialEntries={['/projects/p?slide=s2']}><Routes><Route path="/projects/:projectId" element={<Probe />} /></Routes></MemoryRouter>);
    expect(screen.getByTestId('selection')).toHaveTextContent('s2|?slide=s2');

    act(() => useProjectStore.setState({ contentByProjectId: { p: content(['s0', 's1', 's2', 's3']) } }));
    expect(screen.getByTestId('selection')).toHaveTextContent('s2|?slide=s2');

    act(() => useProjectStore.setState({ contentByProjectId: { p: content(['s0', 's1', 's3']) } }));
    expect(screen.getByTestId('selection')).toHaveTextContent('s3|?slide=s3');
  });
});
