import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { ToolActivityRow } from './ActivityRows';
import { PlanPanel } from './PlanPanel';
import { FinalMessage } from './FinalMessage';
import { QuestionPanel } from './QuestionPanel';
import { LiveProgressRow } from './LiveProgressRow';
import { useProjectStore } from '../../stores/projectStore';
import { useDeckStore } from '../../stores/deckStore';
import { useThreadStore } from '../../stores/threadStore';
import { useRunStore } from '../../stores/runStore';
import type { QuestionItem } from './eventReducer';

describe('public timeline components', () => {
  it('renders a compact tool row without raw args or observations', () => {
    render(<ToolActivityRow item={{
      id: 'r:tool:c1', type: 'tool', runId: 'r', callId: 'c1',
      tool: 'write_ppt', label: '已生成第 3 页', detail: '内容已写入安全暂存区',
      status: 'completed', timestamp: 0,
    }} />);
    expect(screen.getByText('已生成第 3 页')).toBeInTheDocument();
    expect(screen.queryByText(/args|observation|技术详情/)).toBeNull();
  });

  it('renders only controlled render preview URLs and warnings', () => {
    useProjectStore.setState({
      activeProjectId: 'p1',
      slidesByProjectId: {
        p1: [
          { id: 'slide-01', title: '一', position: 0 } as any,
          { id: 'slide-03', title: '三', position: 1 } as any,
        ],
      },
    });
    render(<ToolActivityRow item={{
      id: 'r:tool:c1', type: 'tool', runId: 'r', callId: 'c1',
      tool: 'render_slide', label: '第 3 页渲染通过', status: 'completed', timestamp: 0,
      preview: { slide_id: 'slide-03', image_url: '/api/v1/runs/r/screenshots/shot-1', warnings: ['标题拥挤'] },
    }} />);
    expect(screen.getByAltText('slide-03 渲染预览')).toHaveAttribute(
      'src',
      '/api/v1/runs/r/screenshots/shot-1',
    );
    expect(screen.getByText('1 项布局提示')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '在工作区查看 slide-03' }));
    expect(useDeckStore.getState().currentPage).toBe(1);
  });

  it('renders a flat plan panel and collapses completed plans', () => {
    render(<PlanPanel running={false} plan={{
      id: 'p1', title: '生成演示文稿', revision: 2,
      steps: [{ id: 's1', title: '完成页面', status: 'completed' }],
    }} />);
    expect(screen.getByText('生成演示文稿')).toBeInTheDocument();
    expect(screen.queryByText('完成页面')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: /生成演示文稿/ }));
    expect(screen.getByText('完成页面')).toBeInTheDocument();
  });

  it('renders final as an ordinary agent message with affected target footer', () => {
    render(<FinalMessage item={{
      id: 'f1', type: 'final', messageId: 'm1', text: '**整份演示文稿已完成**',
      affectedTargets: [{ type: 'global' }, { type: 'slide', slide_id: 's1' }],
      timestamp: 0,
    }} />);
    expect(screen.getByText('整份演示文稿已完成')).toBeInTheDocument();
    expect(screen.getByText('已更新全局设计和1 张页面')).toBeInTheDocument();
    expect(screen.queryByText('执行结果')).toBeNull();
  });

  it('waits for question.answered before showing an answered state', async () => {
    useProjectStore.setState({ activeProjectId: 'p1' });
    useThreadStore.setState({ activeThreadIdByProjectId: { p1: 't1' } });
    const answerQuestion = vi.fn().mockResolvedValue(true);
    useRunStore.setState({
      answerQuestion,
      sessions: {
        t1: {
          activeRunId: 'r1', status: 'waiting',
          target: { artifact: 'presentation', level: 'deck' },
          interaction: { intent: 'ask' }, timelineItems: [],
          pendingQuestion: { id: 'q1', prompt: '选择风格' },
          progress: null, eventSourceClose: null, plan: null,
        },
      },
    });
    const item: QuestionItem = {
      id: 'r1:question:q1', type: 'question', runId: 'r1', questionId: 'q1',
      prompt: '选择风格', selection: 'single',
      options: [{ id: 'tech', label: '克制科技', description: '深色背景' }],
      allowCustom: false, timestamp: 0,
    };
    const { rerender } = render(<QuestionPanel item={item} />);
    fireEvent.click(screen.getByLabelText(/克制科技/));
    fireEvent.click(screen.getByRole('button', { name: /提交/ }));
    await waitFor(() => expect(answerQuestion).toHaveBeenCalledWith(
      't1', 'r1', 'q1',
      JSON.stringify({ selected_option_ids: ['tech'], custom_text: '' }),
    ));
    expect(screen.queryByText(/你选择了/)).toBeNull();

    rerender(<QuestionPanel item={{
      ...item,
      answer: { selected_option_ids: ['tech'], custom_text: '' },
      displayText: '克制科技',
    }} />);
    await waitFor(() => expect(screen.getByText('你选择了：克制科技')).toBeInTheDocument());
  });

  it('renders progress as an aria-live row', () => {
    render(<LiveProgressRow progress={{
      stage: 'rendering', text: '正在检查第 6 页的布局', current: 6, total: 12,
    }} />);
    expect(screen.getByText('正在检查第 6 页的布局').closest('[aria-live="polite"]')).toBeInTheDocument();
    expect(screen.getByText('6 / 12')).toBeInTheDocument();
    expect(document.querySelector('.motion-reduce\\:animate-none')).toBeInTheDocument();
  });
});
