import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import type { SlideSpec } from '../../api/types';
import { useProjectStore } from '../../stores/projectStore';
import { SlideSpecCard } from './SlideSpecCard';

// Test the Spec edit/save contract independently of Radix pointer APIs absent in jsdom.
vi.mock('../../components/ui/select', () => ({
  Select: ({ id, value, onValueChange, options }: { id?: string; value: string; onValueChange: (value: string) => void; options: { value: string; label: string }[] }) =>
    <select id={id} value={value} onChange={event => onValueChange(event.target.value)}>
      {options.map(option => <option key={option.value} value={option.value}>{option.label}</option>)}
    </select>,
}));

const spec: SlideSpec = {
  role: 'evidence',
  key_message: '投入正在转为正式预算',
  elements: [{ type: 'chart', intent: '用数字与趋势图展示连续增长' }],
  layout: 'data-story',
};

describe('SlideSpecCard', () => {
  it('renders semantic spec fields without synchronization status', () => {
    render(<SlideSpecCard title="预算正在增长" spec={spec} />);
    expect(screen.getByRole('region', { name: '页面设计稿' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: '页面设计稿' })).toBeInTheDocument();
    expect(screen.getByText('论据')).toBeInTheDocument();
    expect(screen.getByText('预算正在增长')).toBeInTheDocument();
    expect(screen.getByText('投入正在转为正式预算')).toBeInTheDocument();
    expect(screen.getByText('data-story')).toBeInTheDocument();
    expect(screen.getByText('图表')).toBeInTheDocument();
    expect(screen.queryByText('设计稿有更新')).not.toBeInTheDocument();
  });

  it.each([['结论', 'conclusion'], ['未设置', undefined]] as const)('saves the optional role as %s in the page Spec', async (label, role) => {
    const user = userEvent.setup();
    const mutateProject = vi.fn().mockResolvedValue(undefined);
    useProjectStore.setState({ mutateProject });
    render(<SlideSpecCard title="预算正在增长" spec={spec} projectId="p" slideId="sli_a" hash="spec-hash" sceneRevision={1} />);
    await user.click(screen.getByRole('button', { name: '编辑' }));
    await user.selectOptions(screen.getByRole('combobox', { name: '页面角色（选填）' }), screen.getByRole('option', { name: label }));
    await user.click(screen.getByRole('button', { name: '保存' }));
    const expected = { ...spec };
    if (role) expected.role = role; else delete expected.role;
    await waitFor(() => expect(mutateProject).toHaveBeenCalledTimes(1));
    expect(mutateProject).toHaveBeenCalledWith('p', {
      op: 'slide.spec.write', slide_id: 'sli_a', spec: expected, expected_hash: 'spec-hash', expected_scene_revision: 1,
    });
  });

  it('shows an unset role while Spec is pending', () => {
    render(<SlideSpecCard title="新页面" />);
    expect(screen.getByText('未设置')).toBeInTheDocument();
    expect(screen.queryByText('内容')).not.toBeInTheDocument();
  });
});
