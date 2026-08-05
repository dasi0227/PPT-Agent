import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { ToolActivityRow } from './ActivityRows';
import { PlanIndicator } from './PlanIndicator';
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

  it('uses the product term 设计稿 for tool labels supplied by the service', () => {
    render(<ToolActivityRow item={{
      id: 'r:tool:c2', type: 'tool', runId: 'r', callId: 'c2',
      tool: 'read_ppt', label: '已读取全局蓝图', status: 'completed', timestamp: 0,
    }} />);
    expect(screen.getByText('已读取全局设计稿')).toBeInTheDocument();
    expect(screen.queryByText('已读取全局蓝图')).toBeNull();
  });

  it('keeps failed tool details collapsed by default', () => {
    render(<ToolActivityRow item={{
      id: 'r:tool:c3', type: 'tool', runId: 'r', callId: 'c3',
      tool: 'write_ppt', label: '生成页面失败', status: 'failed', timestamp: 0,
      error: { code: 'RENDER_FAILED', message: '页面检查未通过', retryable: true },
    }} />);
    expect(screen.getByText('生成页面失败')).toBeInTheDocument();
    expect(screen.queryByText('页面检查未通过')).toBeNull();
    fireEvent.click(screen.getByRole('button'));
    expect(screen.getByText('页面检查未通过')).toBeInTheDocument();
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
      tool: 'render_slide', label: '已检查页面 slide-03布局', status: 'completed', timestamp: 0,
      target: { type: 'slide', slide_id: 'slide-03', part: 'html' },
      preview: { slide_id: 'slide-03', image_url: '/api/v1/runs/r/screenshots/shot-1', warnings: ['标题拥挤'] },
    }} />);
    expect(screen.getByText('已检查第 2 页布局')).toBeInTheDocument();
    expect(screen.getByAltText('第 2 页渲染预览')).toHaveAttribute(
      'src',
      '/api/v1/runs/r/screenshots/shot-1',
    );
    expect(screen.getByText('第 2 页')).toBeInTheDocument();
    expect(screen.getByText('1 项布局提示')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '在工作区查看 第 2 页' }));
    expect(useDeckStore.getState().currentPage).toBe(1);
  });

  it('shows a compact plan indicator and reveals steps in a popover', () => {
    render(<PlanIndicator running={false} plan={{
      id: 'p1', title: '生成演示文稿', revision: 2,
      steps: [
        { id: 's1', title: '完成页面', status: 'completed' },
        { id: 's2', title: '收尾检查', status: 'pending' },
      ],
    }} />);
    const trigger = screen.getByRole('button', { name: '计划 1 / 2' });
    expect(screen.queryByText('完成页面')).toBeNull();
    fireEvent.pointerDown(trigger, { button: 0, ctrlKey: false });
    fireEvent.click(trigger);
    expect(screen.getByText('生成演示文稿')).toBeInTheDocument();
    expect(screen.getByText('完成页面')).toBeInTheDocument();
    expect(screen.getByText('收尾检查')).toBeInTheDocument();
  });

	  it('shows a plan mode button when no plan exists', () => {
	    const selectPlan = vi.fn();
	    render(<PlanIndicator running={false} plan={null} selected={false} onSelectPlan={selectPlan} />);
	    const trigger = screen.getByRole('button', { name: '计划' });
	    expect(trigger).toHaveAttribute('aria-pressed', 'false');
	    fireEvent.click(trigger);
	    expect(selectPlan).toHaveBeenCalledTimes(1);
	  });

  it('renders final as an ordinary agent message with affected target footer', () => {
    render(<FinalMessage item={{
      id: 'f1', type: 'final', messageId: 'm1', text: '**整份演示文稿已完成**',
      affectedTargets: [
        { type: 'deck', part: 'design' },
        { type: 'slide', slide_id: 's1', part: 'spec' },
        { type: 'slide', slide_id: 's1', part: 'html' },
      ],
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
      allowCustom: false,
      questions: [{
        id: 'question-1', title: '选择风格',
        options: [{ id: 'tech', label: '克制科技', description: '深色背景' }],
        allow_custom: false,
      }],
      grouped: false,
      timestamp: 0,
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
    await waitFor(() => expect(screen.getByText('A：')).toBeInTheDocument());
    expect(screen.getByText('克制科技')).toBeInTheDocument();
  });

  it('requires every grouped question before submitting', async () => {
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
          pendingQuestion: { id: 'q2', prompt: '题型' },
          progress: null, eventSourceClose: null, plan: null,
        },
      },
    });
    const item: QuestionItem = {
      id: 'r1:question:q2', type: 'question', runId: 'r1', questionId: 'q2',
      prompt: '题型', selection: 'single', options: [], allowCustom: true,
      questions: [
        {
          id: 'type', title: '有明确选项时，题型如何处理？',
          options: [{ id: 'single', label: '单选题' }],
          allow_custom: false,
        },
        {
          id: 'icon', title: '如果选项可能不完整，如何允许用户补充？',
          options: [{ id: 'msg', label: 'MessageCircleQuestion' }],
          allow_custom: true,
        },
        { id: 'note', title: '还有哪些约束？', options: [], allow_custom: true },
      ],
      grouped: true,
      timestamp: 0,
    };
    render(<QuestionPanel item={item} />);
    const submit = screen.getByRole('button', { name: /提交回答/ });
    expect(submit).toBeDisabled();
    fireEvent.click(screen.getByLabelText(/单选题/));
    fireEvent.click(screen.getByRole('button', { name: '下一个问题' }));
    fireEvent.click(screen.getByLabelText(/自定义回答/));
    fireEvent.change(screen.getByPlaceholderText('输入自定义回答'), { target: { value: '其他问题图标' } });
    fireEvent.click(screen.getByRole('button', { name: '下一个问题' }));
    fireEvent.change(screen.getByPlaceholderText('输入你的回答'), { target: { value: '保持简洁' } });
    expect(submit).not.toBeDisabled();
    fireEvent.click(submit);
    await waitFor(() => expect(answerQuestion).toHaveBeenCalledWith(
      't1', 'r1', 'q2',
      JSON.stringify({
        selected_option_ids: [],
        custom_text: '',
        answers: [
          { question_id: 'type', selected_option_id: 'single' },
          { question_id: 'icon', custom_text: '其他问题图标' },
          { question_id: 'note', custom_text: '保持简洁' },
        ],
      }),
    ));
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
