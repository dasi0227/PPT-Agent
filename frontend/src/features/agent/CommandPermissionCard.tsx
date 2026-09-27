import { interactionCardClassName, interactionTitleClassName, interactionReasonClassName, interactionInsetClassName } from './interactionCardStyles';
import { Check, ShieldCheck, ShieldX, X } from 'lucide-react';
import { useState } from 'react';
import { useRunStore } from '../../stores/runStore';
import type { CommandPermissionItem } from './eventReducer';
import { useActiveThreadId } from './useActiveSession';

function AnsweredPermission({ answer }: { answer: 'allow_once' | 'deny' }) {
  const allowed = answer === 'allow_once';
  const Icon = allowed ? ShieldCheck : ShieldX;
  return (
    <div className="grid min-h-[34px] grid-cols-[18px_minmax(0,1fr)] items-center gap-2 px-1.5 py-1 text-[13px] text-text-600">
      <Icon
        className={`h-4 w-4 ${allowed ? 'text-success' : 'text-warning'}`}
        strokeWidth={1.75}
        aria-hidden="true"
      />
      <span>{allowed ? '已允许命令执行，正在继续处理' : '已拒绝命令执行，Agent 将选择其他方式'}</span>
    </div>
  );
}

export function CommandPermissionCard({ item }: { item: CommandPermissionItem }) {
  const threadId = useActiveThreadId();
  const answerCommandPermission = useRunStore((state) => state.answerCommandPermission);
  const [submitting, setSubmitting] = useState<'allow_once' | 'deny' | null>(null);

  if (item.answer) return <AnsweredPermission answer={item.answer} />;

  const submit = async (decision: 'allow_once' | 'deny') => {
    if (!threadId || !item.runId || submitting) return;
    setSubmitting(decision);
    const accepted = await answerCommandPermission(
      threadId,
      item.runId,
      item.interactionId,
      item.callId,
      item.commandHash,
      decision,
    );
    if (!accepted) setSubmitting(null);
  };

  return (
    <article className={`${interactionCardClassName} overflow-hidden`}>
      <div className="px-4 pb-4 pt-[15px]">
        <div className="min-w-0">
          <h3 className={interactionTitleClassName}>命令需要授权执行</h3>
          <p className={interactionReasonClassName}>{item.reason}</p>
          <code className={`${interactionInsetClassName} block overflow-x-auto whitespace-pre-wrap break-words font-mono text-[11px] font-normal leading-[1.55] text-text-800`}>
            {item.command}
          </code>
        </div>
      </div>
      <div className="flex justify-end gap-[7px] border-t border-border px-[11px] py-[9px]" role="group" aria-label="命令授权操作">
        <button
          type="button"
          disabled={submitting !== null}
          onClick={() => void submit('deny')}
          className="inline-flex h-[30px] items-center justify-center gap-1 rounded-md border border-danger/20 bg-danger-soft px-[11px] text-xs font-semibold text-[rgb(var(--ui-danger-hover))] ui-danger disabled:cursor-not-allowed disabled:opacity-50"
        >
          <X className="h-3.5 w-3.5" strokeWidth={1.9} aria-hidden="true" />
          {submitting === 'deny' ? '提交中' : '拒绝'}
        </button>
        <button
          type="button"
          disabled={submitting !== null}
          onClick={() => void submit('allow_once')}
          className="inline-flex h-[30px] items-center justify-center gap-1 rounded-md border border-success/20 bg-success-soft px-[11px] text-xs font-semibold text-success ui-success disabled:cursor-not-allowed disabled:opacity-50"
        >
          <Check className="h-3.5 w-3.5" strokeWidth={1.9} aria-hidden="true" />
          {submitting === 'allow_once' ? '提交中' : '批准'}
        </button>
      </div>
    </article>
  );
}
