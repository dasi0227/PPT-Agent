import React, { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { ArrowRight, ChevronDown, ChevronLeft, ChevronRight, MessageCircleQuestion } from 'lucide-react';
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
        <MessageCircleQuestion
          className={cn('mt-0.5 h-4 w-4 shrink-0 text-success', pending && active && 'animate-pulse motion-reduce:animate-none')}
          strokeWidth={1.75}
        />
        <div className="min-w-0 flex-1">
          <h3 className="text-[14px] font-semibold leading-5 text-text-900">{question.title}</h3>
          {question.description && (
            <p className="mt-1 line-clamp-3 text-[13px] leading-5 text-text-600">{question.description}</p>
          )}
        </div>
      </div>

      {question.options.length > 0 ? (
        <div className="mt-3 space-y-2 pl-6">
          {question.options.slice(0, 3).map((option) => {
            const checked = draft.selectedOptionId === option.id;
            return (
              <label
                key={option.id}
                className={cn(
                  'flex cursor-pointer items-start gap-2 rounded-lg border px-3 py-2 transition-colors',
                  checked ? 'border-border-strong bg-panel-muted' : 'border-border bg-surface',
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
          {question.allow_custom && (
            <label
              className={cn(
                'flex cursor-pointer items-start gap-2 rounded-lg border px-3 py-2 transition-colors',
                draft.selectedOptionId === CUSTOM_OPTION_ID ? 'border-border-strong bg-panel-muted' : 'border-border bg-surface',
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
                className="mt-1 accent-text-900"
              />
              <span className="min-w-0 flex-1">
                <span className="block text-[13px] font-medium text-text-900">自定义回答</span>
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
                  className="mt-2 h-8 w-full rounded-md border border-border bg-surface px-2 text-[13px] focus:outline-none focus-visible:ring-2 focus-visible:ring-accent/40"
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
          className="mt-3 h-28 w-full resize-none rounded-lg border border-border bg-surface px-3 py-2 text-[13px] leading-5 focus:outline-none focus-visible:ring-2 focus-visible:ring-accent/40"
        />
      )}
    </div>
  );
}

function SubmittedQuestionRow({ question, value }: { question: QuestionField; value: string }) {
  const [expanded, setExpanded] = useState(false);
  const toggle = () => setExpanded((current) => !current);
  return (
    <div
      role="button"
      tabIndex={0}
      aria-expanded={expanded}
      onClick={toggle}
      onKeyDown={(event) => {
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault();
          toggle();
        }
      }}
      className="flex cursor-pointer items-start gap-2 rounded-lg px-1.5 py-1 text-[13px] leading-5 text-text-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
    >
      <MessageCircleQuestion className="mt-0.5 h-4 w-4 shrink-0 text-success" strokeWidth={1.75} />
      <div className="min-w-0 flex-1">
        <div className="grid grid-cols-[max-content_minmax(0,1fr)] gap-x-1">
          <span className="font-medium text-text-400">Q：</span>
          <span className="truncate">{question.title}</span>
        </div>
        {expanded && <div className="grid grid-cols-[max-content_minmax(0,1fr)] gap-x-1">
          <span className="font-medium text-text-400">A：</span>
          <span>{value}</span>
        </div>}
      </div>
      <span className="mt-0.5 shrink-0 text-text-400" aria-hidden="true">
        {expanded
          ? <ChevronDown className="h-3.5 w-3.5" strokeWidth={1.75} />
          : <ChevronRight className="h-3.5 w-3.5" strokeWidth={1.75} />}
      </span>
    </div>
  );
}

function SubmittedQuestionDetailRow({ question, value }: { question: QuestionField; value: string }) {
  return (
    <div className="question-detail-row flex items-start gap-2 rounded-lg px-1.5 py-1 text-[13px] leading-5 text-text-900">
      <MessageCircleQuestion
        className="mt-0.5 h-4 w-4 shrink-0 text-success"
        strokeWidth={1.75}
        aria-hidden="true"
      />
      <div className="min-w-0 flex-1">
        <div className="grid grid-cols-[max-content_minmax(0,1fr)] gap-x-1">
          <span className="font-medium text-text-400">Q：</span>
          <span>{question.title}</span>
        </div>
        <div className="grid grid-cols-[max-content_minmax(0,1fr)] gap-x-1">
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
  const [answeredGroupExpanded, setAnsweredGroupExpanded] = useState(false);
  const [viewportHeight, setViewportHeight] = useState<number>();
  const panelRef = useRef<HTMLFieldSetElement>(null);
  const questionRefs = useRef<Array<HTMLDivElement | null>>([]);
  const pending = pendingQuestion?.id === item.questionId && !item.answer;
  const currentQuestion = questions[Math.min(currentIndex, questions.length - 1)];
  const complete = questions.every((question) => hasAnswer(question, drafts[question.id]));

  useEffect(() => {
    setCurrentIndex(0);
    setDrafts(initialDrafts(questions));
    setAnsweredGroupExpanded(false);
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

  if (item.answer) {
    const groupedAnswers = item.answer.answers ?? [];
    if (questions.length > 1) {
      return (
        <div className="rounded-lg transition-colors duration-150">
          <button
            type="button"
            aria-expanded={answeredGroupExpanded}
            onClick={() => setAnsweredGroupExpanded((value) => !value)}
            className="flex min-h-8 w-full items-center gap-2 rounded-lg px-1.5 py-1 text-left text-[13px] text-text-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
          >
            <MessageCircleQuestion className="h-4 w-4 shrink-0 text-success" strokeWidth={1.75} />
            <span className="min-w-0 flex-1 truncate">询问了 {questions.length} 个问题</span>
            {answeredGroupExpanded
              ? <ChevronDown className="h-3.5 w-3.5 shrink-0 text-text-400" strokeWidth={1.75} />
              : <ChevronRight className="h-3.5 w-3.5 shrink-0 text-text-400" strokeWidth={1.75} />}
          </button>
          {answeredGroupExpanded && (
            <div className="space-y-1 pb-1">
              {questions.map((question) => (
                <SubmittedQuestionDetailRow
                  key={question.id}
                  question={question}
                  value={answerText(question, groupedAnswers.find((answer) => answer.question_id === question.id), item)}
                />
              ))}
            </div>
          )}
        </div>
      );
    }
    return (
      <div className="space-y-1">
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

      <div className="flex items-center justify-between border-t border-border px-3 py-2.5">
        {questions.length > 1 ? (
          <div className="inline-flex items-center gap-2 text-[13px] text-text-600">
            <button
              type="button"
              aria-label="上一个问题"
              disabled={currentIndex === 0}
              onClick={() => go(-1)}
              className="inline-flex h-7 w-7 items-center justify-center rounded-md text-text-600 hover:bg-panel-muted disabled:opacity-35"
            >
              <ChevronLeft className="h-4 w-4" strokeWidth={1.75} />
            </button>
            <span className="min-w-[38px] text-center tabular-nums">{currentIndex + 1} / {questions.length}</span>
            <button
              type="button"
              aria-label="下一个问题"
              disabled={currentIndex === questions.length - 1}
              onClick={() => go(1)}
              className="inline-flex h-7 w-7 items-center justify-center rounded-md text-text-600 hover:bg-panel-muted disabled:opacity-35"
            >
              <ChevronRight className="h-4 w-4" strokeWidth={1.75} />
            </button>
          </div>
        ) : <span />}
        <button
          type="button"
          aria-label="继续"
          disabled={!pending || submitting || !complete}
          onClick={() => void submit()}
          className="inline-flex h-9 items-center gap-1 rounded-lg bg-text-900 px-3 text-sm text-surface disabled:cursor-not-allowed disabled:opacity-40"
        >
          {submitting ? '提交中' : '继续'}
          {!submitting && <ArrowRight className="h-4 w-4" strokeWidth={1.75} />}
        </button>
      </div>
    </fieldset>
  );
};
