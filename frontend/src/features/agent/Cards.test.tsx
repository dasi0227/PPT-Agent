import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { ToolCallCard } from './ToolCallCard';
import { ThoughtCard } from './ThoughtCard';
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
    expect(screen.getByText('my_tool')).toBeInTheDocument();
    
    rerender(<ToolCallCard item={{ ...baseItem, status: 'success', observation: 'done' }} />);
    // Expand to see observation
    fireEvent.click(screen.getByText('my_tool'));
    expect(screen.getByText(/done/)).toBeInTheDocument();
  });

  it('ThoughtCard is collapsible', () => {
    render(<ThoughtCard item={{ id: '1', type: 'thought', text: 'I am thinking deeply', timestamp: 0 }} />);
    expect(screen.queryByText('I am thinking deeply')).not.toBeInTheDocument();
    fireEvent.click(screen.getByText('执行思路'));
    expect(screen.getByText('I am thinking deeply')).toBeInTheDocument();
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
    expect(screen.getByText('最终交付')).toBeInTheDocument();
    expect(screen.getByText('已更新第 3 页标题')).toBeInTheDocument();
  });

  it('FinalResultCard renders structured result (generate)', () => {
    render(<FinalResultCard item={{
      id: '1', type: 'final_result', timestamp: 0,
      result: { project_id: 'p1', slide_count: 8, theme: 'swiss-modern', signature: '链路脉冲', design_spec_ref: 'design/design-spec.json', warnings: [] },
    }} />);
    expect(screen.getByText('最终交付')).toBeInTheDocument();
    expect(screen.getByText('8')).toBeInTheDocument();
    expect(screen.getByText('swiss-modern')).toBeInTheDocument();
    expect(screen.getByText('链路脉冲')).toBeInTheDocument();
  });

  it('FinalResultCard lists failed pages from warnings', () => {
    render(<FinalResultCard item={{
      id: '1', type: 'final_result', timestamp: 0,
      result: {
        project_id: 'p1', slide_count: 4, theme: 'project-custom', signature: 'sig',
        warnings: [{ page_index: 1, code: 'FIX_EXCEEDED', message: '经 2 轮修复仍不合规' }],
      },
    }} />);
    expect(screen.getByText('1 项告警')).toBeInTheDocument();
    expect(screen.getByText('第 2 页')).toBeInTheDocument();
    expect(screen.getByText('[FIX_EXCEEDED]')).toBeInTheDocument();
    expect(screen.getByText('经 2 轮修复仍不合规')).toBeInTheDocument();
  });

  it('NeedsInputCard renders and handles input', () => {
    useProjectStore.setState({ activeProjectId: 'p1' });
    useThreadStore.setState({ activeThreadIdByProjectId: { p1: 't1' } });
    useRunStore.setState({
      sessions: {
        t1: {
          activeRunId: 'r1', status: 'needs_input', mode: 'ask', scope: 'current',
          timelineItems: [], progress: null, eventSourceClose: null, plan: null,
          pendingInput: { id: '1', prompt: 'Select one', choices: ['A', 'B'] },
        },
      },
    });
    render(<NeedsInputCard item={{ id: '1', type: 'needs_input', prompt: 'Select one', choices: ['A', 'B'], timestamp: 0 }} />);
    expect(screen.getByText('Select one')).toBeInTheDocument();
    expect(screen.getByText('A')).toBeInTheDocument();
    expect(screen.getByText('B')).toBeInTheDocument();
  });
});
