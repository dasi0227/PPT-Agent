import { act, cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, expect, it } from 'vitest';
import type { PublicTarget } from '../../api/types';
import { useProjectStore } from '../../stores/projectStore';
import { FinalChangeSummary } from './FinalMessage';
import { ChangeDiffCard } from './ChangeDiffCard';

const target: PublicTarget = {
  type: 'deck', part: 'manifest', insertions: 2, deletions: 2,
  diff: { kind: 'fields', status: 'modified', filename: '.manifest.json', fields: [
    { field: 'title', rows: [{ kind: 'removed', value: '"旧标题"' }, { kind: 'added', value: '"新标题"' }] },
    { field: 'requirements', rows: [{ kind: 'removed', value: '"旧要求"' }, { kind: 'added', value: '"新要求"' }] },
  ] },
};
afterEach(cleanup);
it('shows frozen field values and semantic counts without current project content', () => {
  act(() => useProjectStore.setState({ activeProjectId: null, contentByProjectId: {} }));
  render(<FinalChangeSummary targets={[target]} />);
  fireEvent.click(screen.getByRole('button', { name: '1 项内容已更改' }));
  fireEvent.click(screen.getByRole('button', { name: /内容要求.*\+2.*−2/ }));
  expect(screen.getByText('旧标题')).toBeInTheDocument();
  expect(screen.getByText('新标题')).toBeInTheDocument();
  act(() => useProjectStore.setState({ activeProjectId: 'another', contentByProjectId: {} }));
  expect(screen.getByText('新标题')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: '预览' })).toBeDisabled();
});
it('shows old and new source lines safely for a deleted page', () => {
  const { container } = render(<ChangeDiffCard target={{ type: 'slide', slide_id: 'sli_a', part: 'html', deletions: 1,
    diff: { kind: 'text', status: 'deleted', filename: 'sli_a.html', hunks: [{ old_start: 4, old_count: 1, new_start: 3, new_count: 0,
      rows: [{ kind: 'removed', old_line: 4, text: '<script>alert("x")</script>' }] }] } }} />);
  expect(screen.getByText('4')).toBeInTheDocument();
  expect(screen.getByText('<script>alert("x")</script>')).toBeInTheDocument();
  expect(container.querySelector('script')).toBeNull();
  expect(screen.getByRole('button', { name: '预览' })).toBeDisabled();
});
it('keeps two changed project files as two summary entries', () => {
  render(<FinalChangeSummary targets={['a.txt', 'b.txt'].map(filename => ({ type: 'file', part: 'content', display_name: filename,
    diff: { kind: 'text', status: 'added', filename, hunks: [] } }))} />);
  fireEvent.click(screen.getByRole('button', { name: '2 项内容已更改' }));
  expect(screen.getByRole('button', { name: 'a.txt' })).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'b.txt' })).toBeInTheDocument();
});
