import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { ModelSelector } from './ModelSelector';

describe('ModelSelector', () => {
  it('keeps capability gating without rendering an inline vision warning', () => {
    render(
      <ModelSelector
        profiles={[{
          name: '文本模型',
          provider: 'custom', protocol: 'responses', model: 'text-model',
          capabilities: { vision: false, tool_calls: true, multiple_tool_calls: true, context_window_tokens: 65536 },
        }]}
        value="文本模型"
        requiresVision
        loading={false}
        onChange={vi.fn()}
      />,
    );

    const trigger = screen.getByRole('button', { name: '模型' });
    expect(trigger).toHaveTextContent('文本模型');
    expect(trigger.querySelector('svg')).toBeInTheDocument();
    expect(trigger).toHaveClass('composer-model-button');
    expect(screen.queryByText('此模型不支持页面观察')).toBeNull();
  });

  it('shows only selectable models in the menu', async () => {
    const user = userEvent.setup();
    render(
      <ModelSelector
        profiles={[
          {
            name: 'DeepSeek Flash',
            provider: 'custom', protocol: 'responses', model: 'deepseek-flash',
            capabilities: { vision: false, tool_calls: true, multiple_tool_calls: true, context_window_tokens: 65536 },
          },
          {
            name: 'Kimi K2',
            provider: 'custom', protocol: 'responses', model: 'kimi-k2',
            capabilities: { vision: false, tool_calls: true, multiple_tool_calls: true, context_window_tokens: 65536 },
          },
        ]}
        value="DeepSeek Flash"
        requiresVision={false}
        loading={false}
        onChange={vi.fn()}
      />,
    );

    await user.click(screen.getByRole('button', { name: '模型' }));
    expect(screen.queryByText('使用主路默认模型')).not.toBeInTheDocument();
    expect(screen.getAllByRole('menuitemradio')).toHaveLength(2);
  });
});
