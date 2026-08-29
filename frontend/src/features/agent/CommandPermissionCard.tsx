import { Check, ShieldCheck, ShieldQuestion, ShieldX, X } from 'lucide-react';
import { useState } from 'react';
import { useRunStore } from '../../stores/runStore';
import type { CommandPermissionItem } from './eventReducer';
import { useActiveThreadId } from './useActiveSession';

function AnsweredPermission({ answer }: { answer: 'allow_once' | 'deny' }) {
  const allowed = answer === 'allow_once';
  const Icon = allowed ? ShieldCheck : ShieldX;
  return (
    <div className="grid min-h-[34px] grid-cols-[18px_minmax(0,1fr)] items-center gap-2 px-1.5 py-1 text-[13px] text-text-600 motion-safe:animate-[timeline-enter_120ms_ease-out]">
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
    <article className="overflow-hidden rounded-[9px] border border-[#E9C98F] bg-surface shadow-[0_2px_8px_rgb(58_46_25_/_7%)] motion-safe:animate-[timeline-enter_120ms_ease-out]">
      <div className="grid grid-cols-[20px_minmax(0,1fr)] gap-[9px] px-3 pb-2.5 pt-3">
        <ShieldQuestion className="mt-px h-[18px] w-[18px] text-warning" strokeWidth={1.75} aria-hidden="true" />
        <div className="min-w-0">
          <h3 className="text-[13px] font-bold leading-5 text-text-900">命令等待授权执行</h3>
          <code className="mt-[9px] block overflow-x-auto whitespace-pre-wrap break-words rounded-md border border-text-400/20 bg-[#F7F8FA] px-[9px] py-2 font-mono text-[11px] font-semibold leading-[1.55] text-[#263241]">
            {item.command}
          </code>
          <p className="mt-1 text-xs leading-[1.55] text-text-600">{item.reason}</p>
        </div>
      </div>
      <div className="flex justify-end gap-[7px] border-t border-[#EADFC9] bg-[#FFFAF1] px-[11px] py-[9px]" role="group" aria-label="命令授权操作">
        <button
          type="button"
          disabled={submitting !== null}
          onClick={() => void submit('deny')}
          className="inline-flex h-[30px] items-center justify-center gap-1 rounded-md border border-[#CFD6DF] bg-surface px-[11px] text-xs font-semibold text-text-600 hover:bg-panel-muted disabled:cursor-not-allowed disabled:opacity-50"
        >
          <X className="h-3.5 w-3.5" strokeWidth={1.9} aria-hidden="true" />
          {submitting === 'deny' ? '提交中' : '拒绝'}
        </button>
        <button
          type="button"
          disabled={submitting !== null}
          onClick={() => void submit('allow_once')}
          className="inline-flex h-[30px] items-center justify-center gap-1 rounded-md bg-[#293544] px-[11px] text-xs font-semibold text-white hover:bg-[#1E2835] disabled:cursor-not-allowed disabled:opacity-50"
        >
          <Check className="h-3.5 w-3.5" strokeWidth={1.9} aria-hidden="true" />
          {submitting === 'allow_once' ? '提交中' : '允许一次'}
        </button>
      </div>
    </article>
  );
}
