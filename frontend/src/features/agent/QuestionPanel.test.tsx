import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { QuestionPanel } from './QuestionPanel';
import type { QuestionItem } from './eventReducer';

const { answerQuestion } = vi.hoisted(() => ({ answerQuestion: vi.fn().mockResolvedValue(true) }));
vi.mock('../../stores/runStore', () => ({ useRunStore: (selector: (state: unknown) => unknown) => selector({ answerQuestion }) }));
vi.mock('./useActiveSession', () => ({ useActiveThreadId: () => 'thread', useActiveSession: () => ({ activeRunId: 'run', pendingQuestion: { id: 'questions' } }) }));

const item: QuestionItem = {
  id: 'question-item', type: 'question', runId: 'run', questionId: 'questions', timestamp: 0,
  questions: [{ id: 'q1', question: '采用哪个方案？', reason: '选择会决定页面顺序。', allow_custom: true,
    options: [{ id: 'o1', label: '方案一', description: '先展示结论。' }] }],
};

describe('QuestionPanel answers', () => {
  beforeEach(() => answerQuestion.mockClear());

  it('submits skip explicitly without selecting an option', async () => {
    render(<QuestionPanel item={item} />);
    expect(screen.getByText('选择会决定页面顺序。')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '继续' })).toBeEnabled();
    fireEvent.click(screen.getByRole('button', { name: '继续' }));
    expect(screen.getByRole('radio', { name: /方案一/ })).toHaveFocus();
    expect(screen.getByRole('button', { name: '跳过' })).toHaveClass('bg-warning-soft');
    fireEvent.click(screen.getByRole('button', { name: '跳过' }));
    expect(screen.getByRole('button', { name: '已跳过' })).toHaveAttribute('aria-pressed', 'true');
    fireEvent.click(screen.getByRole('button', { name: '继续' }));
    await waitFor(() => expect(answerQuestion).toHaveBeenCalledWith('thread', 'run', 'questions', JSON.stringify({ answers: [{ question_id: 'q1', skipped: true }] })));
  });
  it('clears skip when the user enters a custom answer and preserves the text', async () => {
    render(<QuestionPanel item={item} />);
    fireEvent.click(screen.getByRole('button', { name: '跳过' }));
    expect(screen.getByRole('button', { name: '已跳过' })).toBeInTheDocument();
    fireEvent.change(screen.getByPlaceholderText('输入自定义回答'), { target: { value: '  保留原文  ' } });
    expect(screen.getByRole('button', { name: '跳过' })).toHaveAttribute('aria-pressed', 'false');
    fireEvent.click(screen.getByRole('button', { name: '继续' }));
    await waitFor(() => expect(answerQuestion).toHaveBeenCalledWith('thread', 'run', 'questions', JSON.stringify({ answers: [{ question_id: 'q1', custom_text: '  保留原文  ' }] })));
  });

  it('shows the skipped state again when returning to a question and can undo it', () => {
    render(<QuestionPanel item={{ ...item, questions: [
      item.questions[0],
      { id: 'q2', question: '还需要补充什么？', reason: '', options: [], allow_custom: true },
    ] }} />);

    fireEvent.click(screen.getByRole('button', { name: '跳过' }));
    expect(screen.getByText('2 / 2')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '上一个问题' }));
    const skippedButton = screen.getByRole('button', { name: '已跳过' });
    expect(skippedButton).toHaveAttribute('aria-pressed', 'true');
    fireEvent.click(skippedButton);
    expect(screen.getByRole('button', { name: '跳过' })).toHaveAttribute('aria-pressed', 'false');
  });

  it('revisits unanswered questions in order before submitting', async () => {
    render(<QuestionPanel item={{ ...item, questions: [
      { id: 'q1', question: '第一题', reason: '', options: [], allow_custom: true },
      { id: 'q2', question: '第二题', reason: '', options: [], allow_custom: true },
      { id: 'q3', question: '第三题', reason: '', options: [], allow_custom: true },
    ] }} />);

    fireEvent.click(screen.getByRole('button', { name: '下一个问题' }));
    fireEvent.change(screen.getByRole('textbox'), { target: { value: '第二题答案' } });
    fireEvent.click(screen.getByRole('button', { name: '下一个问题' }));
    fireEvent.click(screen.getByRole('button', { name: '继续' }));
    expect(screen.getByText('1 / 3')).toBeInTheDocument();
    expect(answerQuestion).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('button', { name: '继续' }));
    expect(screen.getByText('3 / 3')).toBeInTheDocument();
    fireEvent.change(screen.getByRole('textbox'), { target: { value: '第三题答案' } });
    fireEvent.click(screen.getByRole('button', { name: '继续' }));
    expect(screen.getByText('1 / 3')).toBeInTheDocument();
    fireEvent.change(screen.getByRole('textbox'), { target: { value: '第一题答案' } });
    fireEvent.click(screen.getByRole('button', { name: '继续' }));

    await waitFor(() => expect(answerQuestion).toHaveBeenCalledWith('thread', 'run', 'questions', JSON.stringify({ answers: [
      { question_id: 'q1', custom_text: '第一题答案' },
      { question_id: 'q2', custom_text: '第二题答案' },
      { question_id: 'q3', custom_text: '第三题答案' },
    ] })));
  });
});
