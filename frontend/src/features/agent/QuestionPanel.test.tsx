import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { QuestionPanel } from './QuestionPanel';
import type { QuestionItem } from './eventReducer';

const { answerQuestion } = vi.hoisted(() => ({ answerQuestion: vi.fn().mockResolvedValue(true) }));
vi.mock('../../stores/runStore', () => ({ useRunStore: (selector: (state: unknown) => unknown) => selector({ answerQuestion }) }));
vi.mock('./useActiveSession', () => ({ useActiveThreadId: () => 'thread', useActiveSession: () => ({ activeRunId: 'run', pendingQuestion: { id: 'questions' } }) }));

const item: QuestionItem = {
  id: 'question-item', type: 'question', runId: 'run', questionId: 'questions', timestamp: 0,
  questions: [{ id: 'q1', title: '采用哪个方案？', reason: '选择会决定页面顺序。', allow_custom: true,
    options: [{ id: 'o1', label: '方案一', description: '先展示结论。' }] }],
};

describe('QuestionPanel answers', () => {
  it('submits skip explicitly without selecting an option', async () => {
    render(<QuestionPanel item={item} />);
    expect(screen.getByText('选择会决定页面顺序。')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '继续' })).toBeDisabled();
    fireEvent.click(screen.getByRole('button', { name: '跳过此题' }));
    fireEvent.click(screen.getByRole('button', { name: '继续' }));
    await waitFor(() => expect(answerQuestion).toHaveBeenCalledWith('thread', 'run', 'questions', JSON.stringify({ answers: [{ question_id: 'q1', skipped: true }] })));
  });
  it('clears skip when the user enters a custom answer and preserves the text', async () => {
    render(<QuestionPanel item={item} />);
    fireEvent.click(screen.getByRole('button', { name: '跳过此题' }));
    fireEvent.change(screen.getByPlaceholderText('输入自定义回答'), { target: { value: '  保留原文  ' } });
    fireEvent.click(screen.getByRole('button', { name: '继续' }));
    await waitFor(() => expect(answerQuestion).toHaveBeenCalledWith('thread', 'run', 'questions', JSON.stringify({ answers: [{ question_id: 'q1', custom_text: '  保留原文  ' }] })));
  });
});
