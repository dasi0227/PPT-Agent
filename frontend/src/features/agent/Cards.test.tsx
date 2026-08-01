import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { ToolCallCard } from './ToolCallCard';
import { PlanCard } from './PlanCard';
import { FinalResultCard } from './FinalResultCard';
import { NeedsInputCard } from './NeedsInputCard';
import { useRunStore } from '../../stores/runStore';
import { useProjectStore } from '../../stores/projectStore';
import { useThreadStore } from '../../stores/threadStore';

describe('Agent Cards', () => {
  it('ToolCallCard renders running/success/failed states', () => {
    const baseItem = { id: '1', type: 'tool_call' as const, call_id: 'c1', tool: 'my_tool', args: { a: 1 }, timestamp: 0, artifacts: [] };
    
    const { rerender } = render(<ToolCallCard item={{ ...baseItem, status: 'running' }} />);
    expect(screen.getByText('执行项目操作')).toBeInTheDocument();
    fireEvent.click(screen.getByText('技术详情'));
    expect(screen.getByText(/工具：my_tool/)).toBeInTheDocument();
    
    rerender(<ToolCallCard item={{ ...baseItem, status: 'success', observation: 'done' }} />);
    expect(screen.getAllByText(/done/)).toHaveLength(2);
  });

  it('PlanCard renders states', () => {
    const plan = {
      id: 'plan_r1', title: 'My Plan',
      steps: [
        { id: 's1', title: 'Step 1', status: 'completed' as const },
        { id: 's2', title: 'Step 2', status: 'in_progress' as const },
      ],
    };
    render(<PlanCard plan={plan} />);
    expect(screen.getByText('My Plan')).toBeInTheDocument();
    expect(screen.getByText('Step 1')).toBeInTheDocument();
    expect(screen.getByText('Step 2')).toBeInTheDocument();
  });

  it('FinalResultCard renders summary fallback (edit/outline/command)', () => {
    render(<FinalResultCard item={{ id: '1', type: 'final_result', result: { summary: '已更新第 3 页标题' }, timestamp: 0 }} />);
    expect(screen.getByText('执行结果')).toBeInTheDocument();
    expect(screen.getByText('已更新第 3 页标题')).toBeInTheDocument();
  });

  it('FinalResultCard renders string result via markdown', () => {
    const { container } = render(<FinalResultCard item={{ id: '1', type: 'final_result', result: '**bold** and `code`', timestamp: 0 }} />);
    expect(container.querySelector('strong')?.textContent).toBe('bold');
    expect(container.querySelector('code')?.textContent).toBe('code');
  });

  it('FinalResultCard renders structured workflow outcome', () => {
    render(<FinalResultCard item={{
      id: '1', type: 'final_result', timestamp: 0,
      result: {
        status: 'completed', strategy: 'full_pev', operation: 'rebuild',
        target: { artifact: 'presentation', level: 'deck' },
        affected: [{ kind: 'presentation_slide', id: 's1' }], issues: [], summary: '已完成',
      },
    }} />);
    expect(screen.getByText('执行结果')).toBeInTheDocument();
    expect(screen.getByText(/整份HTML/)).toBeInTheDocument();
    expect(screen.getByText(/完整工作流/)).toBeInTheDocument();
    expect(screen.getByText(/页面 HTML/)).toBeInTheDocument();
  });

  it('FinalResultCard lists verifier issues', () => {
    render(<FinalResultCard item={{
      id: '1', type: 'final_result', timestamp: 0,
      result: {
        status: 'completed', strategy: 'compact_workflow',
        target: { artifact: 'presentation', level: 'slide' },
        issues: [{ code: 'OVERFLOW', evidence: '内容溢出' }],
      },
    }} />);
    expect(screen.getByText('1 项需要注意')).toBeInTheDocument();
    fireEvent.click(screen.getByText('查看验证问题'));
    expect(screen.getByText(/\[OVERFLOW\] 内容溢出/)).toBeInTheDocument();
  });

  it('NeedsInputCard renders and enters an answered state after submission', async () => {
    useProjectStore.setState({ activeProjectId: 'p1' });
    useThreadStore.setState({ activeThreadIdByProjectId: { p1: 't1' } });
    useRunStore.setState({
      replyNeedsInput: vi.fn().mockResolvedValue(true),
      sessions: {
        t1: {
          activeRunId: 'r1', status: 'needs_input', target: { artifact: 'presentation', level: 'slide' }, interaction: { intent: 'apply', clarification: 'before_apply' },
          timelineItems: [], progress: null, eventSourceClose: null, plan: null,
          pendingInput: { id: '1', prompt: 'Select one', choices: ['A', 'B'] },
        },
      },
    });
    render(<NeedsInputCard item={{ id: '1', type: 'needs_input', prompt: 'Select one', choices: ['A', 'B'], timestamp: 0 }} />);
    expect(screen.getByText('Select one')).toBeInTheDocument();
    expect(screen.getByText('A')).toBeInTheDocument();
    expect(screen.getByText('B')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'A' }));
    await waitFor(() => expect(screen.getByText('已回答')).toBeInTheDocument());
    expect(screen.queryByRole('button', { name: 'A' })).toBeNull();
  });

  it('NeedsInputCard renders markdown in prompt', () => {
    useProjectStore.setState({ activeProjectId: 'p1' });
    useThreadStore.setState({ activeThreadIdByProjectId: { p1: 't1' } });
    useRunStore.setState({
      sessions: {
        t1: {
          activeRunId: 'r1', status: 'needs_input', target: { artifact: 'presentation', level: 'slide' }, interaction: { intent: 'apply', clarification: 'before_apply' },
          timelineItems: [], progress: null, eventSourceClose: null, plan: null,
          pendingInput: { id: '2', prompt: 'Confirm `delete`?', choices: [] },
        },
      },
    });
    const { container } = render(<NeedsInputCard item={{ id: '2', type: 'needs_input', prompt: 'Confirm `delete`?', choices: [], timestamp: 0 }} />);
    expect(container.querySelector('code')?.textContent).toBe('delete');
  });
});
