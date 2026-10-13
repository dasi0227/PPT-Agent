import { act, render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom';
import { beforeEach, describe, expect, it } from 'vitest';
import type { ProjectContentSnapshot } from '../../api/types';
import { useDeckStore } from '../../stores/deckStore';
import { useProjectStore } from '../../stores/projectStore';
import { useWorkspaceUrlState } from './useWorkspaceUrlState';

function content(ids: string[]): ProjectContentSnapshot {
  return {
    project_id: 'p',
    outline: {
      sections: [{ id: 'section', title: '章节', slides: ids.map((id) => ({ id, title: id })), subsections: [] }],
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

  it('keeps the selected page view while opening a document and returning after page deletion', () => {
    render(<MemoryRouter initialEntries={['/projects/p?slide=s2&view=spec&content=source']}><Routes><Route path="/projects/:projectId" element={<Probe />} /></Routes></MemoryRouter>);
    act(() => useDeckStore.getState().setActiveDocument('outline'));
    expect(screen.getByTestId('document')).toHaveTextContent('outline');
    expect(useDeckStore.getState().globalView).toBe('outline');
    expect(screen.getByTestId('selection')).toHaveTextContent('s2|?document=outline&view=spec&content=source');

    act(() => useProjectStore.setState({ contentByProjectId: { p: content(['s1', 's3']) } }));
    expect(screen.getByTestId('document')).toHaveTextContent('outline');
    expect(screen.getByTestId('selection')).toHaveTextContent('s3|?document=outline&view=spec&content=source');

    act(() => useDeckStore.getState().setCurrentSlideId('s3'));
    expect(screen.getByTestId('document')).toHaveTextContent('slides');
    expect(screen.getByTestId('selection')).toHaveTextContent('s3|?slide=s3&view=spec&content=source');
    expect(useDeckStore.getState().globalView).toBe('outline');
  });

  it('opens project documents before any slides exist and leaves them for overview', () => {
    useProjectStore.setState({ contentByProjectId: { p: content([]) } });
    render(<MemoryRouter initialEntries={['/projects/p?document=manifest&view=spec']}><Routes><Route path="/projects/:projectId" element={<Probe />} /></Routes></MemoryRouter>);
    expect(screen.getByTestId('document')).toHaveTextContent('manifest');
    expect(screen.getByTestId('selection')).toHaveTextContent('|?document=manifest&view=spec');

    act(() => useProjectStore.setState({ contentByProjectId: { p: content(['s1']) } }));
    expect(screen.getByTestId('document')).toHaveTextContent('manifest');
    expect(screen.getByTestId('selection')).toHaveTextContent('s1|?document=manifest&view=spec');

    act(() => useDeckStore.getState().enterOverview());
    expect(screen.getByTestId('document')).toHaveTextContent('slides');
    expect(screen.getByTestId('selection')).toHaveTextContent('s1|?mode=overview&view=spec');
    expect(useDeckStore.getState().globalView).toBe('outline');
  });

  it('opens HTML source from a URL when the previous selection was a project document', () => {
    useDeckStore.setState({ activeDocument: 'manifest', currentSlideId: 's1' });
    render(<MemoryRouter initialEntries={['/projects/p?slide=s1&content=source']}><Routes><Route path="/projects/:projectId" element={<Probe />} /></Routes></MemoryRouter>);
    expect(screen.getByTestId('document')).toHaveTextContent('slides');
    expect(screen.getByTestId('selection')).toHaveTextContent('s1|?slide=s1&view=html&content=source');
    expect(useDeckStore.getState().contentMode).toBe('source');
  });
  it.each([
    ['?slide=s2&view=spec', 'outline', 'preview', null],
    ['?slide=s2&view=spec&content=source', 'outline', 'source', null],
    ['?document=manifest&content=source', 'html', 'source', 'manifest'],
    ['?document=design&view=spec&content=source', 'outline', 'source', 'design'],
  ] as const)('restores %s directly from the URL', (query, view, mode, document) => {
    render(<MemoryRouter initialEntries={[`/projects/p${query}`]}><Routes><Route path="/projects/:projectId" element={<Probe />} /></Routes></MemoryRouter>);
    expect(useDeckStore.getState()).toMatchObject({ globalView: view, contentMode: mode, activeDocument: document });
    expect(screen.getByTestId('selection').textContent?.split('|')[1]).toBe(query);
  });

});
