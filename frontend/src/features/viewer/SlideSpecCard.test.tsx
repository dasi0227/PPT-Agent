import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { ProjectContentSnapshot, SlideSpec } from '../../api/types';
import { useProjectStore } from '../../stores/projectStore';
import { SlideSpecCard } from './SlideSpecCard';

// Native control isolates the save contract from Radix pointer APIs in jsdom.
vi.mock('../../components/ui/select', () => ({
  Select: ({ value, onValueChange, options, ...props }: { value: string; onValueChange: (value: string) => void; options: { value: string; label: string }[] }) =>
    <select {...props} value={value} onChange={event => onValueChange(event.target.value)}>
      {options.map(option => <option key={option.value} value={option.value}>{option.label}</option>)}
    </select>,
}));
const spec: SlideSpec = { purpose: 'content_explanation', core: '投入正在转为正式预算', elements: [{ type: 'chart', intent: '用数字与趋势图展示连续增长' }], layout: 'data-story' };
const mutateProject = vi.fn();
beforeEach(() => {
  mutateProject.mockReset().mockResolvedValue({ hashes: { 'spec:sli_a': 'saved' }, scene_revision: 1 } satisfies Partial<ProjectContentSnapshot>);
  useProjectStore.setState({ mutateProject, contentByProjectId: {} });
});

describe('SlideSpecCard', () => {
  it.each([['结论', { op: 'replace', path: '/purpose', value: 'conclusion' }], ['未设置', { op: 'remove', path: '/purpose' }]] as const)('saves only the purpose change when selecting %s', async (label, patch) => {
    const user = userEvent.setup();
    render(<SlideSpecCard title="预算正在增长" spec={spec} projectId="p" slideId="sli_a" hash="spec-hash" sceneRevision={1} />);
    const purposeSelect = screen.getByRole('combobox', { name: '页面用途' });
    await user.selectOptions(purposeSelect, within(purposeSelect).getByRole('option', { name: label }));
    await waitFor(() => expect(mutateProject).toHaveBeenCalledTimes(1));
    expect(mutateProject).toHaveBeenCalledWith('p', { op: 'slide.spec.patch', slide_id: 'sli_a', patch: [patch], expected_hash: 'spec-hash', expected_scene_revision: 1 });
  });

  it('creates a missing spec from an element without a core', async () => {
    render(<SlideSpecCard title="新页面" projectId="p" slideId="sli_a" sceneRevision={1} />);
    expect(screen.queryByRole('button', { name: 'JSON 切换' })).not.toBeInTheDocument();
    expect(screen.getByRole('combobox', { name: '页面用途' })).toBeEnabled();
    fireEvent.click(screen.getByRole('button', { name: '新增内容元素' }));
    fireEvent.change(screen.getByLabelText('新增内容元素'), { target: { value: '展示目录' } });
    fireEvent.click(screen.getByRole('button', { name: '保存' }));
    await waitFor(() => expect(mutateProject).toHaveBeenCalledTimes(1));
    expect(mutateProject).toHaveBeenCalledWith('p', { op: 'slide.spec.write', slide_id: 'sli_a', spec: { elements: [{ type: 'text', intent: '展示目录' }] }, expected_hash: undefined, expected_scene_revision: 1 });
  });

  it.each(['content_explanation', 'content_comparison', 'content_example', 'content_guidance', 'content_other'])('saves %s with one selection', async value => {
    const user = userEvent.setup();
    render(<SlideSpecCard title="新页面" spec={{ purpose: 'cover', elements: [] }} projectId="p" slideId="sli_a" hash="spec-hash" sceneRevision={1} />);
    const purpose = screen.getByRole('combobox', { name: '页面用途' });
    expect(screen.queryByRole('combobox', { name: '正文类型' })).not.toBeInTheDocument();
    await user.selectOptions(purpose, value);
    await waitFor(() => expect(mutateProject).toHaveBeenCalledTimes(1));
    expect(mutateProject).toHaveBeenCalledWith('p', {
      op: 'slide.spec.patch', slide_id: 'sli_a',
      patch: [{ op: 'replace', path: '/purpose', value }],
      expected_hash: 'spec-hash', expected_scene_revision: 1,
    });
  });

  it('removes a cleared core from the saved spec', async () => {
    render(<SlideSpecCard title="预算正在增长" spec={spec} projectId="p" slideId="sli_a" hash="spec-hash" sceneRevision={1} />);
    fireEvent.click(screen.getByRole('button', { name: '编辑核心信息' }));
    fireEvent.change(screen.getByLabelText('核心信息'), { target: { value: '' } });
    fireEvent.click(screen.getByRole('button', { name: '保存' }));
    await waitFor(() => expect(mutateProject).toHaveBeenCalledTimes(1));
    expect(mutateProject).toHaveBeenCalledWith('p', {
      op: 'slide.spec.patch', slide_id: 'sli_a', patch: [{ op: 'remove', path: '/core' }],
      expected_hash: 'spec-hash', expected_scene_revision: 1,
    });
  });
});
