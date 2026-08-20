import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { ModelSelector } from './ModelSelector';

describe('ModelSelector', () => {
  it('keeps capability gating without rendering an inline vision warning', () => {
    render(
      <ModelSelector
        profiles={[{
          name: '文本模型',
          model: 'text-model',
          capabilities: { vision: false, tool_calls: true, multiple_tool_calls: true },
        }]}
        value="文本模型"
        requiresVision
        loading={false}
        onChange={vi.fn()}
      />,
    );

    expect(screen.getByRole('button', { name: '模型' })).toHaveTextContent('文本模型');
    expect(screen.queryByText('此模型不支持页面观察')).toBeNull();
  });
});
