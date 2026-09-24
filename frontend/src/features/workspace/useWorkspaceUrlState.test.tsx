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
  const activeDocument = useDeckStore((state) => state.activeDocument);
  const location = useLocation();
  return <><output data-testid="selection">{slideId}|{location.search}</output><output data-testid="document">{activeDocument ?? 'slides'}</output></>;
}

describe('workspace URL state during background content updates', () => {
  beforeEach(() => {
    sessionStorage.clear();
    useDeckStore.setState({ currentSlideId: null, activeDocument: null, globalView: 'html', previewMode: 'main', contentMode: 'preview' });
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

  it('restores a document from the URL and stays there when the remembered page is deleted', () => {
    render(<MemoryRouter initialEntries={['/projects/p?slide=s2&content=source&document=design']}><Routes><Route path="/projects/:projectId" element={<Probe />} /></Routes></MemoryRouter>);
    expect(screen.getByTestId('document')).toHaveTextContent('design');

    act(() => useProjectStore.setState({ contentByProjectId: { p: content(['s1', 's3']) } }));
    expect(screen.getByTestId('document')).toHaveTextContent('design');
    expect(screen.getByTestId('selection')).toHaveTextContent('s3|?slide=s3&content=source&document=design');

    act(() => useDeckStore.getState().setCurrentSlideId('s3'));
    expect(screen.getByTestId('document')).toHaveTextContent('slides');
    expect(screen.getByTestId('selection')).toHaveTextContent('s3|?slide=s3&content=source');
  });

  it('opens project documents before any slides exist and leaves them for overview', () => {
    useProjectStore.setState({ contentByProjectId: { p: content([]) } });
    render(<MemoryRouter initialEntries={['/projects/p?document=manifest']}><Routes><Route path="/projects/:projectId" element={<Probe />} /></Routes></MemoryRouter>);
    expect(screen.getByTestId('document')).toHaveTextContent('manifest');
    expect(screen.getByTestId('selection')).toHaveTextContent('|?document=manifest');

    act(() => useProjectStore.setState({ contentByProjectId: { p: content(['s1']) } }));
    expect(screen.getByTestId('document')).toHaveTextContent('manifest');
    expect(screen.getByTestId('selection')).toHaveTextContent('s1|?slide=s1&document=manifest');

    act(() => useDeckStore.getState().enterOverview());
    expect(screen.getByTestId('document')).toHaveTextContent('slides');
    expect(screen.getByTestId('selection')).toHaveTextContent('s1|?slide=s1&mode=overview');
  });
});
