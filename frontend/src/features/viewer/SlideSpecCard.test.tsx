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
  it.each([['结论', { op: 'replace', path: '/purpose', value: 'conclusion' }]] as const)('saves only the purpose change when selecting %s', async (label, patch) => {
    const user = userEvent.setup();
    render(<SlideSpecCard spec={spec} projectId="p" slideId="sli_a" hash="spec-hash" sceneRevision={1} />);
    const purposeSelect = screen.getByRole('combobox', { name: '页面用途' });
    expect(within(purposeSelect).queryByRole('option', { name: '未设置' })).not.toBeInTheDocument();
    await user.selectOptions(purposeSelect, within(purposeSelect).getByRole('option', { name: label }));
    await waitFor(() => expect(mutateProject).toHaveBeenCalledTimes(1));
    expect(mutateProject).toHaveBeenCalledWith('p', { op: 'slide.spec.patch', slide_id: 'sli_a', patch: [patch], expected_hash: 'spec-hash', expected_scene_revision: 1 });
  });

  it('creates a missing spec only after all required fields are supplied', async () => {
    const user = userEvent.setup();
    render(<SlideSpecCard projectId="p" slideId="sli_a" sceneRevision={1} />);
    expect(screen.queryByRole('button', { name: 'JSON 切换' })).not.toBeInTheDocument();
    const save = screen.getByRole('button', { name: '保存' });
    expect(save).toBeDisabled();
    await user.selectOptions(screen.getByRole('combobox', { name: '页面用途' }), 'introduction');
    fireEvent.change(screen.getByLabelText('核心信息'), { target: { value: '展示目录' } });
    expect(save).toBeDisabled();
    expect(mutateProject).not.toHaveBeenCalled();
    fireEvent.change(screen.getByLabelText('布局建议'), { target: { value: '标题置顶，目录纵向排列' } });
    expect(save).toBeEnabled();
    fireEvent.click(save);
    await waitFor(() => expect(mutateProject).toHaveBeenCalledTimes(1));
    expect(mutateProject).toHaveBeenCalledWith('p', {
      op: 'slide.spec.write', slide_id: 'sli_a',
      spec: { core: '展示目录', purpose: 'introduction', layout: '标题置顶，目录纵向排列', elements: [] },
      expected_hash: undefined, expected_scene_revision: 1,
    });
  });

  it.each(['content_explanation', 'content_comparison', 'content_example', 'content_guidance', 'content_other'])('saves %s with one selection', async value => {
    const user = userEvent.setup();
    render(<SlideSpecCard spec={{ ...spec, purpose: 'cover', elements: [] }} projectId="p" slideId="sli_a" hash="spec-hash" sceneRevision={1} />);
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

  it.each([['core', '核心信息'], ['layout', '布局建议']] as const)('rejects clearing %s and saves a valid replacement', async (field, label) => {
    render(<SlideSpecCard spec={spec} projectId="p" slideId="sli_a" hash="spec-hash" sceneRevision={1} />);
    fireEvent.click(screen.getByRole('button', { name: `编辑${label}` }));
    fireEvent.change(screen.getByLabelText(label), { target: { value: '   ' } });
    fireEvent.click(screen.getByRole('button', { name: '保存' }));
    expect(mutateProject).not.toHaveBeenCalled();
    expect(screen.getByRole('alert')).toHaveTextContent(`请填写${label}`);
    fireEvent.change(screen.getByLabelText(label), { target: { value: '更新内容' } });
    fireEvent.click(screen.getByRole('button', { name: '保存' }));
    await waitFor(() => expect(mutateProject).toHaveBeenCalledTimes(1));
    expect(mutateProject).toHaveBeenCalledWith('p', {
      op: 'slide.spec.patch', slide_id: 'sli_a', patch: [{ op: 'replace', path: `/${field}`, value: '更新内容' }],
      expected_hash: 'spec-hash', expected_scene_revision: 1,
    });
  });
});
