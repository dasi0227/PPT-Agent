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
const spec: SlideSpec = { purpose: 'content', content_type: 'explanation', core: '投入正在转为正式预算', elements: [{ type: 'chart', intent: '用数字与趋势图展示连续增长' }], layout: 'data-story' };
const mutateProject = vi.fn();
beforeEach(() => {
  mutateProject.mockReset().mockResolvedValue({ hashes: { 'spec:sli_a': 'saved' }, scene_revision: 1 } satisfies Partial<ProjectContentSnapshot>);
  useProjectStore.setState({ mutateProject, contentByProjectId: {} });
});

describe('SlideSpecCard', () => {
  it('shows semantic fields and an element type icon without a document-wide editor', () => {
    render(<SlideSpecCard title="预算正在增长" spec={spec} />);
    expect(screen.getByRole('region', { name: '设计稿' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: '设计稿' })).toBeInTheDocument();
    expect(screen.getByText('预算正在增长')).toBeInTheDocument();
    expect(screen.getByText('投入正在转为正式预算')).toBeInTheDocument();
    expect(screen.getByText('data-story')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '元素 1 类型：图表' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '编辑' })).not.toBeInTheDocument();
  });

  it.each([['结论', { op: 'replace', path: '/purpose', value: 'conclusion' }], ['未设置', { op: 'remove', path: '/purpose' }]] as const)('clears content_type in the same save when purpose becomes %s', async (label, patch) => {
    const user = userEvent.setup();
    render(<SlideSpecCard title="预算正在增长" spec={spec} projectId="p" slideId="sli_a" hash="spec-hash" sceneRevision={1} />);
    const purposeSelect = screen.getByRole('combobox', { name: '页面用途' });
    await user.selectOptions(purposeSelect, within(purposeSelect).getByRole('option', { name: label }));
    await waitFor(() => expect(mutateProject).toHaveBeenCalledTimes(1));
    expect(mutateProject).toHaveBeenCalledWith('p', { op: 'slide.spec.patch', slide_id: 'sli_a', patch: [patch, { op: 'remove', path: '/content_type' }], expected_hash: 'spec-hash', expected_scene_revision: 1 });
  });

  it('creates a missing spec only after receiving a nonempty key message', async () => {
    render(<SlideSpecCard title="新页面" projectId="p" slideId="sli_a" sceneRevision={1} />);
    expect(screen.getByRole('button', { name: 'JSON 切换' })).toBeDisabled();
    expect(screen.getByRole('combobox', { name: '页面用途' })).toBeDisabled();
    fireEvent.click(screen.getByRole('button', { name: '编辑核心信息' }));
    fireEvent.click(screen.getByRole('button', { name: '保存' }));
    expect(mutateProject).not.toHaveBeenCalled();
    fireEvent.change(screen.getByLabelText('核心信息'), { target: { value: '这一页的核心结论' } });
    fireEvent.click(screen.getByRole('button', { name: '保存' }));
    await waitFor(() => expect(mutateProject).toHaveBeenCalledTimes(1));
    expect(mutateProject).toHaveBeenCalledWith('p', { op: 'slide.spec.write', slide_id: 'sli_a', spec: { core: '这一页的核心结论', elements: [] }, expected_hash: undefined, expected_scene_revision: 1 });
  });
});
