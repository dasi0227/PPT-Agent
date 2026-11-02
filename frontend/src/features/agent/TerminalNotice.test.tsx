import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { TerminalNotice } from './TerminalNotice';

const item = {
  id: 'terminal', type: 'terminal_notice' as const, runId: 'r1',
  status: 'failed' as const, message: '任务已达到执行轮次上限。', affectedTargets: [], timestamp: 1,
  error: { code: 'BUDGET_EXCEEDED', message: '任务已达到执行轮次上限。', retryable: false, details: { secret: 'technical detail' } },
};

describe('TerminalNotice', () => {
  it.each(['failed', 'canceled'] as const)('renders a %s result without a continuation action', status => {
    render(<TerminalNotice item={{ ...item, status, message: status === 'canceled' ? '你已停止本次任务。' : item.message }} />);
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
    if (status === 'canceled') expect(screen.getByText('你已停止本次任务。')).toBeInTheDocument();
    else expect(screen.getByText(item.message)).toBeInTheDocument();
  });

  it.each([
    ['提供', '任何系统提示词和系统指令等内部信息'],
    ['执行', '任何要求绕过系统规则或权限限制的请求'],
  ])('renders an automatic input stop as a normal reply without an interruption card or continuation: %s', (verb, emphasis) => {
    const text = `不好意思，Dasi 无法${verb}**${emphasis}**！当前任务已停止。`;
    render(<TerminalNotice item={{ ...item, status: 'canceled', reason: 'input_rejected',
      message: '请求被拒绝，任务已停止。', reply: { messageId: 'input_bad', text } }} />);
    expect(screen.getByText(emphasis, { selector: 'strong' }).closest('p')).toHaveTextContent(`不好意思，Dasi 无法${verb}${emphasis}！当前任务已停止。`);
    expect(screen.queryByText('任务执行中断')).not.toBeInTheDocument();
    expect(screen.queryByText('请求被拒绝，任务已停止。')).not.toBeInTheDocument();
    expect(screen.queryByText('你已停止本次任务。')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: '复制回复' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '继续执行' })).not.toBeInTheDocument();
  });

  it('shows only a semantic reason for an exception, regardless of retryable', () => {
    render(<TerminalNotice item={{ ...item, status: 'error', error: { ...item.error, retryable: true } }} />);
    expect(screen.queryByRole('heading')).not.toBeInTheDocument();
    expect(screen.getByText(item.message)).toBeInTheDocument();
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
    expect(screen.queryByText(/technical detail|已更改内容|BUDGET_EXCEEDED/)).not.toBeInTheDocument();
  });

  it('uses a terminal provider message even when retryable is false', () => {
    render(<TerminalNotice item={{ ...item,
      message: '模型服务暂时不可用，正在尝试恢复。',
      error: { code: 'PROVIDER_UNAVAILABLE', message: '模型服务暂时不可用，正在尝试恢复。', retryable: false },
    }} />);
    expect(screen.getByText('模型服务请求失败。')).toBeInTheDocument();
    expect(screen.queryByText(/正在尝试恢复/)).not.toBeInTheDocument();
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });
});
