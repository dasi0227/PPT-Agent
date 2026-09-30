import { interactionCardClassName, interactionTitleClassName } from './interactionCardStyles';
import React, { useEffect, useLayoutEffect, useRef, useState } from 'react';
import { Check, ChevronLeft, ChevronRight, Circle, MessageCircleQuestion } from 'lucide-react';
import { useRunStore } from '../../stores/runStore';
import { cn } from '../../lib/utils';
import { useActiveSession, useActiveThreadId } from './useActiveSession';
import type { QuestionField, QuestionFieldAnswer } from '../../api/types';
import type { QuestionItem } from './eventReducer';

const CUSTOM_OPTION_ID = '__custom__';

interface DraftAnswer {
  skipped?: boolean;
  selectedOptionId?: string;
  customText: string;
}

function initialDrafts(questions: QuestionField[]): Record<string, DraftAnswer> {
  return Object.fromEntries(questions.map((question) => [question.id, { customText: '' }]));
}

function hasAnswer(question: QuestionField, draft: DraftAnswer | undefined): boolean {
  if (!draft) return false;
  if (draft.skipped) return true;
  if (question.options.length === 0) return draft.customText.trim() !== '';
  if (draft.selectedOptionId === CUSTOM_OPTION_ID) return draft.customText.trim() !== '';
  return Boolean(draft.selectedOptionId);
}

function QuestionSlide({
  active,
  draft,
  pending,
  question,
  questionGroupId,
  setDraft,
  submitting,
  slideRef,
}: {
  active: boolean;
  draft: DraftAnswer;
  pending: boolean;
  question: QuestionField;
  questionGroupId: string;
  setDraft: (questionId: string, patch: Partial<DraftAnswer>) => void;
  submitting: boolean;
  slideRef: (element: HTMLDivElement | null) => void;
}) {
  const disabled = !pending || submitting || !active;
  return (
    <div
      ref={slideRef}
      aria-hidden={!active}
      className="w-full shrink-0 px-3 py-3"
    >
      <div className="flex items-start gap-2">
        <div className="min-w-0 flex-1">
          <h3 className={interactionTitleClassName}>{question.title}</h3>
          {question.reason && (
            <p className="mt-1 line-clamp-3 text-[13px] leading-5 text-text-600">{question.reason}</p>
          )}
        </div>
      </div>

      {question.options.length > 0 ? (
        <div className="mt-3 space-y-2">
          {question.options.slice(0, 3).map((option) => {
            const checked = draft.selectedOptionId === option.id;
            return (
              <label
                aria-disabled={disabled}
                key={option.id}
                className={cn(
                  'ui-interactive flex cursor-pointer items-start gap-2 rounded-lg border px-3 py-2 transition-colors',
                  checked ? 'border-accent/20 ui-selected' : 'border-border bg-transparent',
                  disabled && 'cursor-default opacity-70',
                )}
              >
                <input
                  type="radio"
                  name={`${questionGroupId}:${question.id}`}
                  value={option.id}
                  checked={checked}
                  disabled={disabled}
                  onChange={() => setDraft(question.id, { selectedOptionId: option.id, customText: '' })}
                  className="mt-1 accent-accent"
                />
                <span className="min-w-0 flex-1">
                  <span className={cn('block text-[13px] font-medium', checked ? 'text-selected-foreground' : 'text-text-900')}>{option.label}</span>
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
          {question.allow_custom && (
            <label
                aria-disabled={disabled}
              className={cn(
                'ui-interactive flex cursor-pointer items-start gap-2 rounded-lg border px-3 py-2 transition-colors',
                draft.selectedOptionId === CUSTOM_OPTION_ID ? 'border-accent/20 ui-selected' : 'border-border bg-transparent',
                disabled && 'cursor-default opacity-70',
              )}
            >
              <input
                type="radio"
                name={`${questionGroupId}:${question.id}`}
                value={CUSTOM_OPTION_ID}
                checked={draft.selectedOptionId === CUSTOM_OPTION_ID}
                disabled={disabled}
                onChange={() => setDraft(question.id, { selectedOptionId: CUSTOM_OPTION_ID })}
                className="mt-1 accent-accent"
              />
              <span className="min-w-0 flex-1">
                <span className={cn('block text-[13px] font-medium', draft.selectedOptionId === CUSTOM_OPTION_ID ? 'text-selected-foreground' : 'text-text-900')}>自定义回答</span>
                <input
                  type="text"
                  value={draft.customText}
                  disabled={disabled}
                  onFocus={() => setDraft(question.id, { selectedOptionId: CUSTOM_OPTION_ID })}
                  onChange={(event) => setDraft(question.id, {
                    selectedOptionId: CUSTOM_OPTION_ID,
                    customText: event.target.value,
                  })}
                  placeholder="输入自定义回答"
                  className="mt-2 h-8 w-full rounded-md border border-border bg-surface px-2 text-[13px] focus:outline-none"
                />
              </span>
            </label>
          )}
        </div>
      ) : (
        <textarea
          value={draft.customText}
          disabled={disabled}
          onChange={(event) => setDraft(question.id, { customText: event.target.value })}
          placeholder="输入你的回答"
          className="mt-3 h-28 w-full resize-none rounded-lg border border-border bg-surface px-3 py-2 text-[13px] leading-5 focus:outline-none"
        />
      )}
    </div>
  );
}

function AnsweredQuestionOption({
  description,
  label,
  selected,
}: {
  description?: string;
  label: string;
  selected: boolean;
}) {
  return (
    <div
      className={cn(
        'flex items-start gap-2 rounded-lg border px-3 py-2',
        selected ? 'border-accent/20 ui-selected' : 'border-border bg-transparent',
      )}
    >
      {selected
        ? <Check className="mt-0.5 h-4 w-4 shrink-0 text-accent" strokeWidth={1.75} aria-hidden="true" />
        : <Circle className="mt-0.5 h-4 w-4 shrink-0 text-border-strong" strokeWidth={1.75} aria-hidden="true" />}
      <span className="min-w-0 flex-1">
        <span className={cn('block text-[13px] font-medium', selected ? 'text-selected-foreground' : 'text-text-900')}>{label}</span>
        {description && (
          <span className="mt-0.5 block text-xs leading-5 text-text-600">{description}</span>
        )}
      </span>
    </div>
  );
}

function AnsweredQuestionCard({ item }: { item: QuestionItem }) {
  const questions = item.questions;
  const answers = item.answer?.answers ?? [];
  const [expanded, setExpanded] = useState(false);
  const [currentIndex, setCurrentIndex] = useState(0);
  const detailsId = `question-answer-details-${item.questionId}`;
  const currentQuestion = questions[Math.min(currentIndex, questions.length - 1)];
  const currentAnswer = answers.find((answer) => answer.question_id === currentQuestion.id);

  return (
    <div className="rounded-lg">
      <button
        type="button"
        aria-expanded={expanded}
        aria-controls={detailsId}
        onClick={() => setExpanded((value) => !value)}
        className="flex min-h-8 w-full items-center gap-2 rounded-lg px-1.5 py-1 text-left text-[13px] font-normal leading-5 text-text-900 focus-visible:outline-none"
      >
        <MessageCircleQuestion className="h-4 w-4 shrink-0 text-success" strokeWidth={1.75} />
        <span className="min-w-0 flex-1 truncate">询问了 {questions.length} 个问题</span>
        <ChevronRight
          className={cn('h-3.5 w-3.5 shrink-0 text-text-400 transition-transform', expanded && 'rotate-90')}
          strokeWidth={1.75}
          aria-hidden="true"
        />
      </button>
      {expanded && (
        <article
          id={detailsId}
          data-testid="answered-question-card"
          className={`${interactionCardClassName} timeline-detail-card p-4`}
        >
          <div className="flex items-start gap-2">
            <div className="min-w-0 flex-1">
              <h3 className={interactionTitleClassName}>{currentQuestion.title}</h3>
              {currentQuestion.reason && (
                <p className="mt-1 line-clamp-3 text-[13px] leading-5 text-text-600">{currentQuestion.reason}</p>
              )}
            </div>
          </div>
          {currentQuestion.options.length > 0 ? (
            <div className="mt-3 space-y-2">
              {currentQuestion.options.map((option) => (
                <AnsweredQuestionOption
                  key={option.id}
                  label={option.label}
                  description={option.description}
                  selected={currentAnswer?.selected_option_id === option.id}
                />
              ))}
              {currentQuestion.allow_custom && currentAnswer?.custom_text?.trim() && (
                <AnsweredQuestionOption
                  label="自定义回答"
                  description={currentAnswer.custom_text.trim()}
                  selected
                />
              )}
            </div>
          ) : (
            <p className="mt-3 text-[13px] leading-5 text-text-900">{currentAnswer?.custom_text?.trim() ?? ''}</p>
          )}
          {currentAnswer?.skipped && <p className="mt-3 text-xs text-text-600">用户跳过了此问题</p>}
          {questions.length > 1 && (
            <div className="mt-3 flex items-center justify-between border-t border-border pt-3">
              <div className="inline-flex items-center gap-2 text-[13px] text-text-600">
                <button
                  type="button"
                  aria-label="上一个问题"
                  disabled={currentIndex === 0}
                  onClick={() => setCurrentIndex((index) => index - 1)}
                  className="inline-flex h-7 w-7 items-center justify-center rounded-md text-text-600 ui-interactive disabled:opacity-35"
                >
                  <ChevronLeft className="h-4 w-4" strokeWidth={1.75} />
                </button>
                <span className="min-w-[38px] text-center tabular-nums">{currentIndex + 1} / {questions.length}</span>
                <button
                  type="button"
                  aria-label="下一个问题"
                  disabled={currentIndex === questions.length - 1}
                  onClick={() => setCurrentIndex((index) => index + 1)}
                  className="inline-flex h-7 w-7 items-center justify-center rounded-md text-text-600 ui-interactive disabled:opacity-35"
                >
                  <ChevronRight className="h-4 w-4" strokeWidth={1.75} />
                </button>
              </div>
            </div>
          )}
        </article>
      )}
    </div>
  );
}

export const QuestionPanel: React.FC<{ item: QuestionItem }> = ({ item }) => {
  const threadId = useActiveThreadId();
  const { activeRunId, pendingQuestion } = useActiveSession();
  const answerQuestion = useRunStore((state) => state.answerQuestion);
  const questions = item.questions;
  const [currentIndex, setCurrentIndex] = useState(0);
  const [drafts, setDrafts] = useState<Record<string, DraftAnswer>>(() => initialDrafts(questions));
  const [submitting, setSubmitting] = useState(false);
  const [viewportHeight, setViewportHeight] = useState<number>();
  const panelRef = useRef<HTMLFieldSetElement>(null);
  const questionRefs = useRef<Array<HTMLDivElement | null>>([]);
  const pending = pendingQuestion?.id === item.questionId && !item.answer;
  const currentQuestion = questions[Math.min(currentIndex, questions.length - 1)];
  const complete = questions.every((question) => hasAnswer(question, drafts[question.id]));

  useEffect(() => {
    setCurrentIndex(0);
    setDrafts(initialDrafts(questions));
  }, [item.questionId, questions]);

  useEffect(() => {
    if (pending) panelRef.current?.focus();
  }, [pending]);

  useLayoutEffect(() => {
    if (item.answer) return undefined;
    const current = questionRefs.current[currentIndex];
    if (!current) return undefined;
    const measure = () => {
      const nextHeight = current.offsetHeight;
      if (nextHeight > 0) setViewportHeight(nextHeight);
    };
    measure();
    if (typeof ResizeObserver === 'undefined') return undefined;
    const observer = new ResizeObserver(measure);
    observer.observe(current);
    return () => observer.disconnect();
  }, [currentIndex, drafts, item.answer, questions]);

  if (item.answer) return <AnsweredQuestionCard item={item} />;

  const setDraft = (questionId: string, patch: Partial<DraftAnswer>) => {
    setDrafts((current) => ({
      ...current,
      [questionId]: { ...(current[questionId] ?? { customText: '' }), skipped: false, ...patch },
    }));
  };

  const submit = async () => {
    if (!threadId || !activeRunId || !pending || submitting || !complete) return;
    const answers: QuestionFieldAnswer[] = questions.map((question) => {
      const draft = drafts[question.id] ?? { customText: '' };
      if (draft.skipped) return { question_id: question.id, skipped: true };
      if (question.options.length === 0 || draft.selectedOptionId === CUSTOM_OPTION_ID) {
        return { question_id: question.id, custom_text: draft.customText };
      }
      return { question_id: question.id, selected_option_id: draft.selectedOptionId };
    });
    setSubmitting(true);
    await answerQuestion(threadId, activeRunId, item.questionId, JSON.stringify({ answers }));
    setSubmitting(false);
  };

  const go = (offset: number) => {
    setCurrentIndex((index) => Math.max(0, Math.min(questions.length - 1, index + offset)));
  };

  const continueQuestions = () => {
    if (complete) {
      void submit();
      return;
    }
    const unanswered = (question: QuestionField) => !hasAnswer(question, drafts[question.id]);
    const nextIndex = questions.findIndex((question, index) => index > currentIndex && unanswered(question));
    const firstIndex = nextIndex >= 0 ? nextIndex : questions.findIndex(unanswered);
    if (firstIndex === currentIndex) {
      questionRefs.current[currentIndex]?.querySelector<HTMLElement>('input:not(:disabled), textarea:not(:disabled)')?.focus();
    } else if (firstIndex >= 0) {
      setCurrentIndex(firstIndex);
    }
  };

  return (
    <fieldset
      ref={panelRef}
      tabIndex={-1}
      className={`${interactionCardClassName} focus:outline-none`}
    >
      <legend className="sr-only">{currentQuestion.title}</legend>
      <div
        className="overflow-hidden transition-[height] duration-200 motion-reduce:transition-none"
        style={viewportHeight === undefined ? undefined : { height: viewportHeight }}
      >
        <div
          className="flex transition-transform duration-200 ease-out motion-reduce:transition-none"
          style={{ transform: `translate3d(-${currentIndex * 100}%, 0, 0)` }}
        >
          {questions.map((question, index) => (
            <QuestionSlide
              key={question.id}
              active={index === currentIndex}
              draft={drafts[question.id] ?? { customText: '' }}
              pending={pending}
              question={question}
              questionGroupId={item.questionId}
              setDraft={setDraft}
              submitting={submitting}
              slideRef={(element) => { questionRefs.current[index] = element; }}
            />
          ))}
        </div>
      </div>

      <div className="flex items-center justify-between gap-3 border-t border-border px-3 py-2.5">
        {questions.length > 1 ? (
          <div className="inline-flex items-center gap-2 text-[13px] text-text-600">
            <button
              type="button"
              aria-label="上一个问题"
              disabled={currentIndex === 0}
              onClick={() => go(-1)}
              className="inline-flex h-7 w-7 items-center justify-center rounded-md text-text-600 ui-interactive disabled:opacity-35"
            >
              <ChevronLeft className="h-4 w-4" strokeWidth={1.75} />
            </button>
            <span className="min-w-[38px] text-center tabular-nums">{currentIndex + 1} / {questions.length}</span>
            <button
              type="button"
              aria-label="下一个问题"
              disabled={currentIndex === questions.length - 1}
              onClick={() => go(1)}
              className="inline-flex h-7 w-7 items-center justify-center rounded-md text-text-600 ui-interactive disabled:opacity-35"
            >
              <ChevronRight className="h-4 w-4" strokeWidth={1.75} />
            </button>
          </div>
        ) : <span />}
        <div className="flex items-center gap-2">
          <button type="button" disabled={!pending || submitting} aria-pressed={!!drafts[currentQuestion.id]?.skipped}
            onClick={() => {
              if (drafts[currentQuestion.id]?.skipped) {
                setDraft(currentQuestion.id, { skipped: false });
                return;
              }
              setDraft(currentQuestion.id, { skipped: true, selectedOptionId: undefined, customText: '' });
              if (currentIndex < questions.length - 1) go(1);
            }}
            className="ui-warning inline-flex h-9 items-center justify-center rounded-lg border border-warning/20 bg-warning-soft px-3 text-xs font-medium text-[rgb(var(--ui-warning-foreground))] transition-colors focus-visible:underline focus-visible:underline-offset-2 disabled:cursor-not-allowed disabled:opacity-40">
            {drafts[currentQuestion.id]?.skipped ? '已跳过' : '跳过'}
          </button>
          <button
            type="button"
            aria-label="继续"
            disabled={!pending || submitting}
            onClick={continueQuestions}
            className="inline-flex h-9 items-center gap-1 rounded-lg ui-primary px-3 text-xs transition-colors disabled:cursor-not-allowed disabled:opacity-40"
          >
            {submitting ? '提交中' : '继续'}
          </button>
        </div>
      </div>
    </fieldset>
  );
};
