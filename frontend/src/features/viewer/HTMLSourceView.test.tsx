import { act, render, screen } from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { readHTMLSource, type HTMLSourceDocument } from '../../api/htmlSource';
import { HTMLSourceView } from './HTMLSourceView';

vi.mock('../../api/htmlSource', () => ({ readHTMLSource: vi.fn() }));
vi.mock('../../components/HTMLSource', () => ({ HTMLSource: ({ text }: { text: string }) => <pre>{text}</pre> }));

const document = (id: string, content: string, hash = id, scene = 1): HTMLSourceDocument => ({
  project_id: 'p', slide_id: id, path: `${id}.html`, content, source_hash: hash, scene_revision: scene,
});

beforeEach(() => vi.resetAllMocks());

it('ignores a late response from the previous page and hides old HTML on a scene change', async () => {
  let resolveA!: (value: HTMLSourceDocument) => void;
  let resolveB!: (value: HTMLSourceDocument) => void;
  vi.mocked(readHTMLSource).mockImplementationOnce(() => new Promise(resolve => { resolveA = resolve; }))
    .mockImplementationOnce(() => new Promise(resolve => { resolveB = resolve; }))
    .mockImplementationOnce(() => new Promise(() => {}));
  const props = { projectId: 'p', ordinal: 1, available: true, sceneRevision: 1 };
  const { rerender } = render(<HTMLSourceView {...props} slideId="a" hash="a" />);
  rerender(<HTMLSourceView {...props} slideId="b" hash="b" ordinal={2} />);
  await act(async () => resolveB(document('b', '<h1>Page B</h1>')));
  await act(async () => resolveA(document('a', '<h1>Page A</h1>')));
  expect(screen.getByText('<h1>Page B</h1>')).toBeInTheDocument();
  expect(screen.queryByText('<h1>Page A</h1>')).not.toBeInTheDocument();
  rerender(<HTMLSourceView {...props} slideId="b" hash="b" sceneRevision={2} />);
  expect(screen.queryByText('<h1>Page B</h1>')).not.toBeInTheDocument();
  expect(screen.getByText('正在加载源码…')).toBeInTheDocument();
});

it('does not show HTML with an unexpected content hash', async () => {
  vi.mocked(readHTMLSource).mockResolvedValue(document('a', '<h1>Unobserved update</h1>', 'new'));
  render(<HTMLSourceView projectId="p" slideId="a" hash="old" ordinal={1} available sceneRevision={1} />);
  expect(await screen.findByText('页面内容已更新，请刷新后重试。')).toBeInTheDocument();
  expect(screen.queryByText('<h1>Unobserved update</h1>')).not.toBeInTheDocument();
  expect(screen.getByRole('button', { name: '复制源码' })).toBeDisabled();
});
