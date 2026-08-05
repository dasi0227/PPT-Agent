import React, { useEffect, useMemo, useRef, useState } from 'react';
import { MessageCircleQuestion, Send } from 'lucide-react';
import { useRunStore } from '../../stores/runStore';
import { cn } from '../../lib/utils';
import { useActiveSession, useActiveThreadId } from './useActiveSession';
import type { QuestionField, QuestionFieldAnswer, QuestionOption } from '../../api/types';
import type { QuestionItem } from './eventReducer';

const CUSTOM_OPTION_ID = '__custom__';

interface DraftAnswer {
  selectedOptionId?: string;
  customText: string;
}

function optionLabel(options: QuestionOption[], id?: string): string {
  if (!id) return '';
  return options.find((option) => option.id === id)?.label ?? '';
}

function initialDrafts(questions: QuestionField[]): Record<string, DraftAnswer> {
  return Object.fromEntries(questions.map((question) => [question.id, { customText: '' }]));
}

function hasAnswer(question: QuestionField, draft: DraftAnswer | undefined): boolean {
  if (!draft) return false;
  if (question.options.length === 0) return draft.customText.trim() !== '';
  if (draft.selectedOptionId === CUSTOM_OPTION_ID) return draft.customText.trim() !== '';
  return Boolean(draft.selectedOptionId);
}

function answerText(question: QuestionField, answer: QuestionFieldAnswer | undefined, legacy: QuestionItem): string {
  if (answer) {
    return answer.custom_text?.trim() || optionLabel(question.options, answer.selected_option_id);
  }
  if (!legacy.answer) return '';
  const labels = legacy.answer.selected_option_ids
    .map((id) => optionLabel(question.options, id))
    .filter(Boolean);
  if (legacy.answer.custom_text) labels.push(legacy.answer.custom_text);
  return labels.join('；');
}

function SubmittedQuestionRow({ question, value }: { question: QuestionField; value: string }) {
  return (
    <div className="flex items-start gap-2 px-1.5 py-1 text-[13px] leading-5 text-text-900">
      <MessageCircleQuestion className="mt-0.5 h-4 w-4 shrink-0 text-success" strokeWidth={1.75} />
      <div className="min-w-0 flex-1">
        <div className="grid grid-cols-[24px_minmax(0,1fr)] gap-1">
          <span className="font-medium text-text-400">Q：</span>
          <span>{question.title}</span>
        </div>
        <div className="grid grid-cols-[24px_minmax(0,1fr)] gap-1 text-text-600">
          <span className="font-medium text-text-400">A：</span>
          <span>{value}</span>
        </div>
      </div>
    </div>
  );
}

export const QuestionPanel: React.FC<{ item: QuestionItem }> = ({ item }) => {
  const threadId = useActiveThreadId();
  const { activeRunId, pendingQuestion } = useActiveSession();
  const answerQuestion = useRunStore((state) => state.answerQuestion);
  const questions = useMemo(() => item.questions.length > 0 ? item.questions : [{
    id: 'question-1',
    title: item.prompt,
    options: item.options,
    allow_custom: item.allowCustom,
  }], [item.allowCustom, item.options, item.prompt, item.questions]);
  const [currentIndex, setCurrentIndex] = useState(0);
  const [drafts, setDrafts] = useState<Record<string, DraftAnswer>>(() => initialDrafts(questions));
  const [submitting, setSubmitting] = useState(false);
  const panelRef = useRef<HTMLFieldSetElement>(null);
  const pending = pendingQuestion?.id === item.questionId && !item.answer;
  const currentQuestion = questions[Math.min(currentIndex, questions.length - 1)];
  const currentDraft = drafts[currentQuestion.id] ?? { customText: '' };
  const complete = questions.every((question) => hasAnswer(question, drafts[question.id]));

  useEffect(() => {
    setCurrentIndex(0);
    setDrafts(initialDrafts(questions));
  }, [item.questionId, questions]);

  useEffect(() => {
    if (pending) panelRef.current?.focus();
  }, [pending]);

  if (item.answer) {
    const groupedAnswers = item.answer.answers ?? [];
    return (
      <div className="space-y-1">
        {questions.length > 1 && (
          <div className="flex items-start gap-2 px-1.5 py-1 text-[13px] font-medium leading-5 text-text-900">
            <MessageCircleQuestion className="mt-0.5 h-4 w-4 shrink-0 text-success" strokeWidth={1.75} />
            <span>询问了 {questions.length} 个问题</span>
          </div>
        )}
        {questions.map((question) => (
          <SubmittedQuestionRow
            key={question.id}
            question={question}
            value={answerText(question, groupedAnswers.find((answer) => answer.question_id === question.id), item)}
          />
        ))}
      </div>
    );
  }

  const setDraft = (questionId: string, patch: Partial<DraftAnswer>) => {
    setDrafts((current) => ({
      ...current,
      [questionId]: { ...(current[questionId] ?? { customText: '' }), ...patch },
    }));
  };

  const submit = async () => {
    if (!threadId || !activeRunId || !pending || submitting || !complete) return;
    const groupedAnswers: QuestionFieldAnswer[] = questions.map((question) => {
      const draft = drafts[question.id] ?? { customText: '' };
      if (question.options.length === 0 || draft.selectedOptionId === CUSTOM_OPTION_ID) {
        return { question_id: question.id, custom_text: draft.customText.trim() };
      }
      return { question_id: question.id, selected_option_id: draft.selectedOptionId };
    });
    const first = groupedAnswers[0];
    const legacyAnswer = !item.grouped && first
      ? {
          selected_option_ids: first.selected_option_id ? [first.selected_option_id] : [],
          custom_text: first.custom_text ?? '',
        }
      : { selected_option_ids: [], custom_text: '' };
    const answer = item.grouped
      ? { selected_option_ids: [], custom_text: '', answers: groupedAnswers }
      : legacyAnswer;
    setSubmitting(true);
    await answerQuestion(threadId, activeRunId, item.questionId, JSON.stringify(answer));
    setSubmitting(false);
  };

  const go = (offset: number) => {
    setCurrentIndex((index) => Math.max(0, Math.min(questions.length - 1, index + offset)));
  };

  return (
    <fieldset
      ref={panelRef}
      tabIndex={-1}
      className="rounded-[10px] border border-border-strong bg-surface focus:outline-none focus-visible:ring-2 focus-visible:ring-accent/40"
    >
      <legend className="sr-only">{currentQuestion.title}</legend>
      <div className="min-h-[330px] px-3 py-3">
        <div className="flex items-start gap-2">
          <MessageCircleQuestion
            className={cn('mt-0.5 h-4 w-4 shrink-0 text-success', pending && 'animate-pulse motion-reduce:animate-none')}
            strokeWidth={1.75}
          />
          <div className="min-w-0 flex-1">
            <h3 className="text-[14px] font-semibold leading-5 text-text-900">{currentQuestion.title}</h3>
            {currentQuestion.description && (
              <p className="mt-1 line-clamp-3 text-[13px] leading-5 text-text-600">{currentQuestion.description}</p>
            )}
          </div>
        </div>

        {currentQuestion.options.length > 0 ? (
          <div className="mt-3 space-y-2 pl-6">
            {currentQuestion.options.slice(0, 3).map((option) => {
              const checked = currentDraft.selectedOptionId === option.id;
              return (
                <label
                  key={option.id}
                  className={cn(
                    'flex cursor-pointer items-start gap-2 rounded-lg border px-3 py-2',
                    checked ? 'border-border-strong bg-panel-muted' : 'border-border bg-surface',
                    !pending && 'cursor-default opacity-70',
                  )}
                >
                  <input
                    type="radio"
                    name={`${item.questionId}:${currentQuestion.id}`}
                    value={option.id}
                    checked={checked}
                    disabled={!pending || submitting}
                    onChange={() => setDraft(currentQuestion.id, { selectedOptionId: option.id, customText: '' })}
                    className="mt-1 accent-text-900"
                  />
                  <span className="min-w-0 flex-1">
                    <span className="block text-[13px] font-medium text-text-900">{option.label}</span>
                    {option.description && (
                      <span
                        title={option.description}
                        className="mt-0.5 block line-clamp-2 text-xs leading-5 text-text-600"
                      >
                        {option.description}
                      </span>
                    )}
                  </span>
                </label>
              );
            })}
            {currentQuestion.allow_custom && (
              <label
                className={cn(
                  'flex cursor-pointer items-start gap-2 rounded-lg border px-3 py-2',
                  currentDraft.selectedOptionId === CUSTOM_OPTION_ID ? 'border-border-strong bg-panel-muted' : 'border-border bg-surface',
                  !pending && 'cursor-default opacity-70',
                )}
              >
                <input
                  type="radio"
                  name={`${item.questionId}:${currentQuestion.id}`}
                  value={CUSTOM_OPTION_ID}
                  checked={currentDraft.selectedOptionId === CUSTOM_OPTION_ID}
                  disabled={!pending || submitting}
                  onChange={() => setDraft(currentQuestion.id, { selectedOptionId: CUSTOM_OPTION_ID })}
                  className="mt-1 accent-text-900"
                />
                <span className="min-w-0 flex-1">
                  <span className="block text-[13px] font-medium text-text-900">自定义回答</span>
                  <input
                    type="text"
                    value={currentDraft.customText}
                    disabled={!pending || submitting}
                    onFocus={() => setDraft(currentQuestion.id, { selectedOptionId: CUSTOM_OPTION_ID })}
                    onChange={(event) => setDraft(currentQuestion.id, {
                      selectedOptionId: CUSTOM_OPTION_ID,
                      customText: event.target.value,
                    })}
                    placeholder="输入自定义回答"
                    className="mt-2 h-8 w-full rounded-md border border-border bg-surface px-2 text-[13px] focus:outline-none focus-visible:ring-2 focus-visible:ring-accent/40"
                  />
                </span>
              </label>
            )}
          </div>
        ) : (
          <textarea
            value={currentDraft.customText}
            disabled={!pending || submitting}
            onChange={(event) => setDraft(currentQuestion.id, { customText: event.target.value })}
            placeholder="输入你的回答"
            className="mt-3 h-36 w-[calc(100%-1.5rem)] resize-none rounded-lg border border-border bg-surface px-3 py-2 text-[13px] leading-5 focus:outline-none focus-visible:ring-2 focus-visible:ring-accent/40"
          />
        )}
      </div>

      <div className="flex items-center justify-between border-t border-border px-3 py-2.5">
        {questions.length > 1 ? (
          <div className="inline-flex items-center gap-2 text-[13px] text-text-600">
            <button
              type="button"
              aria-label="上一个问题"
              disabled={currentIndex === 0}
              onClick={() => go(-1)}
              className="px-1 text-base disabled:opacity-35"
            >
              &lt;
            </button>
            <span className="min-w-[38px] text-center tabular-nums">{currentIndex + 1} / {questions.length}</span>
            <button
              type="button"
              aria-label="下一个问题"
              disabled={currentIndex === questions.length - 1}
              onClick={() => go(1)}
              className="px-1 text-base disabled:opacity-35"
            >
              &gt;
            </button>
          </div>
        ) : <span />}
        <button
          type="button"
          aria-label="提交回答"
          disabled={!pending || submitting || !complete}
          onClick={() => void submit()}
          className="inline-flex h-9 items-center gap-1 rounded-lg bg-text-900 px-3 text-sm text-surface disabled:cursor-not-allowed disabled:opacity-40"
        >
          <Send className="h-4 w-4" strokeWidth={1.75} /> 提交回答
        </button>
      </div>
    </fieldset>
  );
};
