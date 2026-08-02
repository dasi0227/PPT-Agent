import React, { useEffect, useRef, useState } from 'react';
import { CheckCircle2, MessageCircleQuestion, Send } from 'lucide-react';
import { useRunStore } from '../../stores/runStore';
import { cn } from '../../lib/utils';
import { useActiveSession, useActiveThreadId } from './useActiveSession';
import type { QuestionItem } from './eventReducer';

export const QuestionPanel: React.FC<{ item: QuestionItem }> = ({ item }) => {
  const threadId = useActiveThreadId();
  const { activeRunId, pendingQuestion } = useActiveSession();
  const answerQuestion = useRunStore((state) => state.answerQuestion);
  const [selected, setSelected] = useState<string[]>([]);
  const [customText, setCustomText] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [showQuestion, setShowQuestion] = useState(!item.answer);
  const panelRef = useRef<HTMLFieldSetElement>(null);
  const pending = pendingQuestion?.id === item.questionId && !item.answer;

  useEffect(() => {
    if (pending) panelRef.current?.focus();
  }, [pending]);
  useEffect(() => {
    if (item.answer) setShowQuestion(false);
  }, [item.answer]);

  if (item.answer && !showQuestion) {
    return (
      <div className="rounded-[10px] border border-border bg-surface px-3 py-2">
        <div className="flex items-center gap-2 text-[13px] text-text-900">
          <CheckCircle2 className="h-4 w-4 text-success" strokeWidth={1.75} />
          <span className="min-w-0 flex-1">你选择了：{item.displayText}</span>
          <button type="button" className="text-xs text-accent" onClick={() => setShowQuestion(true)}>查看问题</button>
        </div>
      </div>
    );
  }

  const toggleOption = (id: string) => {
    if (item.selection === 'single') {
      setSelected([id]);
      return;
    }
    setSelected((current) => current.includes(id)
      ? current.filter((value) => value !== id)
      : [...current, id]);
  };

  const submit = async () => {
    if (!threadId || !activeRunId || !pending || submitting) return;
    const answer = {
      selected_option_ids: selected,
      custom_text: customText.trim(),
    };
    if (answer.selected_option_ids.length === 0 && !answer.custom_text) return;
    setSubmitting(true);
    await answerQuestion(threadId, activeRunId, item.questionId, JSON.stringify(answer));
    setSubmitting(false);
  };

  return (
    <fieldset
      ref={panelRef}
      tabIndex={-1}
      className="rounded-[10px] border border-border-strong bg-surface p-3 focus:outline-none focus-visible:ring-2 focus-visible:ring-accent/40"
    >
      <legend className="sr-only">{item.header || '需要你的选择'}</legend>
      <div className="mb-2 flex items-center gap-2 text-xs font-medium text-text-600">
        <MessageCircleQuestion className="h-4 w-4 text-accent" strokeWidth={1.75} />
        {item.header || '需要你的选择'}
      </div>
      <p className="text-sm font-medium leading-6 text-text-900">{item.prompt}</p>

      <div className="mt-3 space-y-2">
        {item.options.map((option) => {
          const checked = selected.includes(option.id) || Boolean(item.answer?.selected_option_ids.includes(option.id));
          return (
            <label
              key={option.id}
              className={cn(
                'flex cursor-pointer items-start gap-2 rounded-lg border px-3 py-2',
                checked ? 'border-accent/40 bg-accent-soft' : 'border-border hover:bg-panel-muted',
                !pending && 'cursor-default opacity-70',
              )}
            >
              <input
                type={item.selection === 'single' ? 'radio' : 'checkbox'}
                name={item.questionId}
                value={option.id}
                checked={checked}
                disabled={!pending || submitting}
                onChange={() => toggleOption(option.id)}
                className="mt-1 accent-accent"
              />
              <span className="min-w-0">
                <span className="block text-[13px] font-medium text-text-900">{option.label}</span>
                {option.description && <span className="mt-0.5 block text-xs leading-5 text-text-600">{option.description}</span>}
              </span>
            </label>
          );
        })}
      </div>

      {item.allowCustom && pending && (
        <div className="mt-3 flex gap-2">
          <input
            type="text"
            value={customText}
            disabled={submitting}
            onChange={(event) => setCustomText(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Enter') {
                event.preventDefault();
                void submit();
              }
            }}
            placeholder="补充你的想法…"
            className="min-w-0 flex-1 rounded-lg border border-border bg-surface px-3 py-2 text-sm focus:outline-none focus-visible:ring-2 focus-visible:ring-accent/40"
          />
          <button
            type="button"
            aria-label="提交回答"
            disabled={submitting || (selected.length === 0 && customText.trim() === '')}
            onClick={() => void submit()}
            className="inline-flex h-9 items-center gap-1 rounded-lg bg-accent px-3 text-sm text-white disabled:opacity-50"
          >
            <Send className="h-4 w-4" strokeWidth={1.75} /> 提交
          </button>
        </div>
      )}

      {!item.allowCustom && pending && (
        <button
          type="button"
          disabled={submitting || selected.length === 0}
          onClick={() => void submit()}
          className="mt-3 inline-flex h-9 items-center gap-1 rounded-lg bg-accent px-3 text-sm text-white disabled:opacity-50"
        >
          <Send className="h-4 w-4" strokeWidth={1.75} /> 提交
        </button>
      )}

      {item.answer && showQuestion && (
        <button type="button" className="mt-3 text-xs text-accent" onClick={() => setShowQuestion(false)}>收起问题</button>
      )}
    </fieldset>
  );
};
