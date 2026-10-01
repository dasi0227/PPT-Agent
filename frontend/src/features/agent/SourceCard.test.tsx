import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { readHTMLSource } from '../../api/htmlSource';
import { repositoriesApi } from '../../api/repositories';
import type { ProjectContentSnapshot } from '../../api/types';
import { useProjectStore } from '../../stores/projectStore';
import { useDeckStore } from '../../stores/deckStore';
import { ResourceSourceCard, TargetSourceCard } from './SourceCard';
import { FinalChangeSummary } from './FinalMessage';
import { ProjectDocumentView } from '../viewer/ProjectDocumentView';

vi.mock('../../api/htmlSource', () => ({ readHTMLSource: vi.fn() }));
vi.mock('../../api/repositories', () => ({ repositoriesApi: { getSkill: vi.fn(), getComponent: vi.fn() } }));
vi.mock('../viewer/htmlSourceFormatClient', () => ({ formatHTMLForDisplay: async (source: string) => {
  const { formatHTMLSource } = await import('../viewer/htmlSourceFormatter');
  return formatHTMLSource(source);
} }));

const snapshot: ProjectContentSnapshot = {
  project_id: 'p1', theme: '', appearance: null, hashes: {},
  manifest: { title: '测试内容', goal: '', audience: '', language: '', pages: '', requirements: [], prohibitions: [] },
  design: { requirements: [], decorations: { page_number: 'bottom-right', section_title: 'top-left', deck_title: 'none', key_message: 'none' } },
  outline: { sections: [{ id: 'sec1', title: '', purpose: '', subsections: [], slides: [{ slide_id: 's1', title: '第一' }, { slide_id: 's2', title: '第二' }] }] },
  slides_by_id: {},
};
function setup() { act(() => useProjectStore.setState({ activeProjectId: 'p1', contentByProjectId: { p1: snapshot } })); }
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
  act(() => {
    useProjectStore.setState({ activeProjectId: null, contentByProjectId: {} });
    useDeckStore.setState({ activeDocument: null, currentSlideId: null, previewMode: 'main', contentMode: 'preview', globalView: 'html' });
  });
});

describe('timeline source cards', () => {
  it('shows indented current JSON, updates with the document, and opens its preview', () => {
    setup();
    const { container } = render(<TargetSourceCard target={{ type: 'deck', part: 'manifest' }} />);
    expect(container.querySelector('code')?.textContent).toBe(JSON.stringify(snapshot.manifest, null, 2));
    expect(screen.getByText('.manifest.json')).toBeInTheDocument();
    act(() => useProjectStore.setState({ contentByProjectId: { p1: { ...snapshot, manifest: { ...snapshot.manifest, title: '更新后' } } } }));
    expect(container.querySelector('code')?.textContent).toContain('更新后');
    fireEvent.click(screen.getByRole('button', { name: '预览' }));
    expect(useDeckStore.getState()).toMatchObject({ activeDocument: 'manifest', contentMode: 'preview' });
  });

  it('opens the outline document and navigates from it to a page specification', () => {
    setup();
    const { rerender } = render(<TargetSourceCard target={{ type: 'deck', part: 'outline' }} />);
    fireEvent.click(screen.getByRole('button', { name: '预览' }));
    expect(useDeckStore.getState().activeDocument).toBe('outline');
    rerender(<ProjectDocumentView document="outline" snapshot={snapshot} onRetry={() => {}} />);
    fireEvent.click(screen.getByRole('button', { name: /第二/ }));
    expect(useDeckStore.getState()).toMatchObject({ activeDocument: null, currentSlideId: 's2', globalView: 'outline' });
  });

  it('ignores an old HTML response after the target changes and formats the new source', async () => {
    setup();
    let finishOld!: (value: Awaited<ReturnType<typeof readHTMLSource>>) => void;
    vi.mocked(readHTMLSource).mockImplementationOnce(() => new Promise(resolve => { finishOld = resolve; }));
    vi.mocked(readHTMLSource).mockResolvedValueOnce({ project_id: 'p1', slide_id: 's2', content: '<section><h1>第二页</h1><p>正文</p></section>', path: '', source_hash: 'new', scene_revision: 1 });
    const { container, rerender } = render(<TargetSourceCard target={{ type: 'slide', part: 'html', slide_id: 's1' }} />);
    rerender(<TargetSourceCard target={{ type: 'slide', part: 'html', slide_id: 's2' }} />);
    await waitFor(() => expect(container.querySelector('code')?.textContent).toContain('\n  <h1>第二页</h1>'));
    await act(async () => finishOld({ project_id: 'p1', slide_id: 's1', content: '旧页面', path: '', source_hash: 'old', scene_revision: 1 }));
    expect(container.textContent).not.toContain('旧页面');
    act(() => useDeckStore.setState({ previewMode: 'overview', activeDocument: 'design', contentMode: 'source' }));
    fireEvent.click(screen.getByRole('button', { name: '预览' }));
    expect(useDeckStore.getState()).toMatchObject({ currentSlideId: 's2', activeDocument: null, previewMode: 'main', globalView: 'html', contentMode: 'preview' });
  });

  it('offers retry after a source request fails', async () => {
    setup();
    vi.mocked(readHTMLSource).mockRejectedValueOnce(new Error('加载失败')).mockResolvedValueOnce({ project_id: 'p1', slide_id: 's1', content: '<p>恢复</p>', path: '', source_hash: 'ok', scene_revision: 1 });
    render(<TargetSourceCard target={{ type: 'slide', part: 'html', slide_id: 's1' }} />);
    fireEvent.click(await screen.findByRole('button', { name: '重试' }));
    expect(await screen.findByText('<p>恢复</p>')).toBeInTheDocument();
  });

  it('shows raw skill text and links to that repository entry', async () => {
    vi.mocked(repositoriesApi.getSkill).mockResolvedValue({ id: 'skill a', name: '技能', description: '', content: '# 标题\n\n原文', disabled: false, open_url: '', content_state: 'ready' });
    const { container } = render(<MemoryRouter><ResourceSourceCard resource={{ kind: 'skill', id: 'skill a', name: '技能' }} /></MemoryRouter>);
    await waitFor(() => expect(container.querySelector('code')?.textContent).toBe('# 标题\n\n原文'));
    expect(screen.getByRole('link', { name: '预览技能' })).toHaveAttribute('href', '/warehouse/skill?id=skill%20a');
  });

  it('uses a frozen diff inside the final change summary', () => {
    setup();
    render(<FinalChangeSummary targets={[{ type: 'deck', part: 'manifest', diff: { kind: 'fields', status: 'modified', filename: '.manifest.json', fields: [{ field: 'title', rows: [{ kind: 'removed', value: JSON.stringify('旧标题') }, { kind: 'added', value: JSON.stringify('新标题') }] }] } }]} />);
    fireEvent.click(screen.getByRole('button', { name: /1 项内容已更改/ }));
    fireEvent.click(screen.getByRole('button', { name: '内容要求' }));
    expect(screen.getByRole('region', { name: '变更差异' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '预览' })).toBeInTheDocument();
    expect(screen.queryByText('外部打开')).not.toBeInTheDocument();
  });
});
