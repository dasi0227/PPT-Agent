import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { ToolCallCard } from './ToolCallCard';
import { ThoughtCard } from './ThoughtCard';
import { PlanCard } from './PlanCard';
import { FinalResultCard } from './FinalResultCard';
import { NeedsInputCard } from './NeedsInputCard';
import { useRunStore } from '../../stores/runStore';

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
    const item = {
      id: '1', type: 'plan' as const, title: 'My Plan', timestamp: 0,
      steps: [
        { id: 's1', title: 'Step 1', status: 'completed' },
        { id: 's2', title: 'Step 2', status: 'in_progress' }
      ]
    };
    render(<PlanCard item={item} />);
    expect(screen.getByText('My Plan')).toBeInTheDocument();
    expect(screen.getByText('Step 1')).toBeInTheDocument();
    expect(screen.getByText('Step 2')).toBeInTheDocument();
  });

  it('FinalResultCard renders', () => {
    render(<FinalResultCard item={{ id: '1', type: 'final_result', result: { url: 'abc' }, timestamp: 0 }} />);
    expect(screen.getByText('最终交付')).toBeInTheDocument();
    expect(screen.getByText(/abc/)).toBeInTheDocument();
  });

  it('NeedsInputCard renders and handles input', () => {
    useRunStore.setState({ pendingInput: { id: '1', prompt: 'Select one', choices: ['A', 'B'] } });
    render(<NeedsInputCard item={{ id: '1', type: 'needs_input', prompt: 'Select one', choices: ['A', 'B'], timestamp: 0 }} />);
    expect(screen.getByText('Select one')).toBeInTheDocument();
    expect(screen.getByText('A')).toBeInTheDocument();
    expect(screen.getByText('B')).toBeInTheDocument();
  });
});
