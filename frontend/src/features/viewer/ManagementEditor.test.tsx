import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { useProjectStore } from '../../stores/projectStore';
import { ManagementEditor, TextField } from './ManagementEditor';

const mutate = vi.fn();
beforeEach(() => {
  mutate.mockReset().mockResolvedValue(undefined);
  useProjectStore.setState({ mutateProject: mutate });
});

function Editor({ hash = 'original', scene = 1 }: { hash?: string; scene?: number }) {
  return <ManagementEditor projectId="p" value={{ key_message: 'Initial', elements: [] }} hash={hash} sceneRevision={scene}
    mutation={spec => ({ op: 'slide.spec.write', slide_id: 'sli_a', spec })}
    fields={(value, change) => <TextField label="核心信息" value={value.key_message} minLength={1} maxLength={500} onChange={key_message => change({ ...value, key_message })} />}>
    <p>Saved content</p>
  </ManagementEditor>;
}

it('keeps a draft but blocks saving when the history scene changes even with the same hash', () => {
  const { rerender } = render(<Editor />);
  fireEvent.click(screen.getByRole('button', { name: '编辑' }));
  fireEvent.change(screen.getByLabelText('核心信息'), { target: { value: 'My draft' } });
  rerender(<Editor scene={2} />);
  expect(screen.getByLabelText('核心信息')).toHaveValue('My draft');
  expect(screen.getByRole('button', { name: '保存' })).toBeDisabled();
  expect(screen.getByText('内容已更新，请取消后重新编辑。')).toBeInTheDocument();
  expect(mutate).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole('button', { name: '取消' }));
  expect(screen.getByText('Saved content')).toBeInTheDocument();
});

it('saves a single business object with the hash and scene captured when editing began', async () => {
  render(<Editor />);
  fireEvent.click(screen.getByRole('button', { name: '编辑' }));
  fireEvent.change(screen.getByLabelText('核心信息'), { target: { value: 'Updated' } });
  fireEvent.click(screen.getByRole('button', { name: '保存' }));
  await waitFor(() => expect(screen.getByRole('button', { name: '编辑' })).toBeInTheDocument());
  expect(mutate).toHaveBeenCalledWith('p', { op: 'slide.spec.write', slide_id: 'sli_a', spec: { key_message: 'Updated', elements: [] }, expected_hash: 'original', expected_scene_revision: 1 });
});
