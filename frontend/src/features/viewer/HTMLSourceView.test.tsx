import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { readHTMLSource, type HTMLSourceDocument } from '../../api/htmlSource';
import { HTMLSourceView } from './HTMLSourceView';
import { formatHTMLForDisplay } from './htmlSourceFormatClient';

vi.mock('../../api/htmlSource', () => ({ readHTMLSource: vi.fn() }));
vi.mock('../../components/HTMLSource', () => ({ HTMLSource: ({ text }: { text: string }) => <pre>{text}</pre> }));
vi.mock('./htmlSourceFormatClient', () => ({ formatHTMLForDisplay: vi.fn() }));

const document = (id: string, content: string, hash = id, scene = 1): HTMLSourceDocument => ({
  project_id: 'p', slide_id: id, path: `${id}.html`, content, source_hash: hash, scene_revision: scene,
});

beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(formatHTMLForDisplay).mockImplementation(async source => source);
});

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

it('displays formatted HTML while copying the original source', async () => {
  const original = '<main><h1>Hello</h1></main>';
  const formatted = '<main>\n  <h1>Hello</h1>\n</main>\n';
  const writeText = vi.fn().mockResolvedValue(undefined);
  Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } });
  vi.mocked(readHTMLSource).mockResolvedValue(document('a', original));
  vi.mocked(formatHTMLForDisplay).mockResolvedValue(formatted);

  render(<HTMLSourceView projectId="p" slideId="a" hash="a" ordinal={1} available sceneRevision={1} />);
  const preview = await screen.findByText((_, element) => element?.tagName === 'PRE');
  expect(preview.textContent).toBe(formatted);
  expect(formatHTMLForDisplay).toHaveBeenCalledWith(original, 'a');
  fireEvent.click(screen.getByRole('button', { name: '复制源码' }));
  expect(writeText).toHaveBeenCalledWith(original);
});

it('ignores a late formatting result from the previous page', async () => {
  let resolveA!: (value: string) => void;
  let resolveB!: (value: string) => void;
  vi.mocked(readHTMLSource).mockImplementation(async (_projectId, slideId) => document(slideId, `<main>${slideId}</main>`));
  vi.mocked(formatHTMLForDisplay).mockImplementation((source) => new Promise((resolve) => {
    if (source.includes('>a<')) resolveA = resolve;
    else resolveB = resolve;
  }));

  const props = { projectId: 'p', ordinal: 1, available: true, sceneRevision: 1 };
  const { rerender } = render(<HTMLSourceView {...props} slideId="a" hash="a" />);
  await waitFor(() => expect(formatHTMLForDisplay).toHaveBeenCalledTimes(1));
  rerender(<HTMLSourceView {...props} slideId="b" hash="b" ordinal={2} />);
  await waitFor(() => expect(formatHTMLForDisplay).toHaveBeenCalledTimes(2));

  await act(async () => resolveB('formatted B'));
  expect(screen.getByText('formatted B')).toBeInTheDocument();
  await act(async () => resolveA('formatted A'));
  expect(screen.queryByText('formatted A')).not.toBeInTheDocument();
  expect(screen.getByText('formatted B')).toBeInTheDocument();
});
