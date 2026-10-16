import { useEffect, useId, useRef, useState, type ReactNode } from 'react';
import { File, Loader2, SquareTerminal } from 'lucide-react';
import type { ToolActivityItem } from './eventReducer';
import { cn } from '../../lib/utils';
import { useRunStore } from '../../stores/runStore';
import { useActiveThreadId } from './useActiveSession';
import { TimelineChevron, TimelineDisclosure } from './TimelineDisclosure';
import { timelineDetailCardClassName } from './interactionCardStyles';
import { HumanInterventionBadge, DecisionControls, decisionFeedback } from './HumanIntervention';
import { ChangeDiffCard } from './ChangeDiffCard';
import { ChangeStats } from './ChangeStats';

function CommandGroup({ label, children }: { label: string; children: ReactNode }) {
  const [open, setOpen] = useState(false);
  const id = useId();
  return <section className="border-t border-border">
    <button type="button" aria-expanded={open} aria-controls={id} onClick={() => setOpen(value => !value)}
      className="ui-interactive flex min-h-10 w-full items-center gap-2 px-3.5 py-2 text-left text-xs text-text-700">
      <TimelineChevron open={open} /><span>{label}</span>
    </button>
    <TimelineDisclosure id={id} open={open}>{open && children}</TimelineDisclosure>
  </section>;
}

function CommandFile({ target }: { target: NonNullable<ToolActivityItem['changes']>[number] }) {
  const [open, setOpen] = useState(false);
  const id = useId();
  const diff = target.diff;
  return <section className="border-t border-border first:border-t-0">
    <button type="button" aria-expanded={open} aria-controls={id} onClick={() => setOpen(value => !value)}
      className="ui-interactive flex min-h-10 w-full items-center gap-2 px-3.5 py-2 text-left text-[11px] text-text-600">
      <TimelineChevron open={open} /><File className="h-3.5 w-3.5 shrink-0" strokeWidth={1.75} />
      <span className="min-w-0 flex-1 truncate font-mono">{diff?.filename ?? target.display_name}</span>
      <span className={cn("shrink-0", diff?.status === 'added' ? 'text-success' : diff?.status === 'deleted' ? 'text-danger' : undefined)}>{diff?.status === 'added' ? '新增' : diff?.status === 'deleted' ? '删除' : '修改'}</span>
      {diff?.status === 'modified' && diff.kind === 'text' && <ChangeStats insertions={target.insertions} deletions={target.deletions} />}
    </button>
    <TimelineDisclosure id={id} open={open}>{open && <ChangeDiffCard target={target} previewEnabled={false} variant="inline" />}</TimelineDisclosure>
  </section>;
}

export function RunCommandActivity({ item }: { item: ToolActivityItem }) {
  const approval = item.commandApproval;
  const pending = Boolean(approval && !approval.answer && item.status === 'running');
  const refused = item.status === 'refuse' || approval?.answer?.decision === 'deny';
  const failed = item.status === 'failed' || item.status === 'blocked';
  const [expanded, setExpanded] = useState(pending);
  const wasPending = useRef(pending);
  const [decision, setDecision] = useState<'allow_once' | 'deny' | ''>('');
  const [feedback, setFeedback] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const threadId = useActiveThreadId();
  const answerCommandPermission = useRunStore(state => state.answerCommandPermission);
  const id = useId();
  useEffect(() => {
    if (pending) setExpanded(true);
    else if (wasPending.current) setExpanded(false);
    wasPending.current = pending;
  }, [pending]);
  const source = item.command?.text ?? '';
  const lines = source.split(/\r?\n/);
  const firstLine = lines[0] + (lines.length > 1 ? ' …' : '');
  const name = source.trim().match(/^\S+/)?.[0];
  const commandLabel = name ? `${name} 命令` : '命令';
  const saving = item.status === 'running' && approval?.answer?.decision === 'allow_once';
  const label = failed ? `执行 ${commandLabel}失败` : refused ? `未应用 ${commandLabel}的更改`
    : pending ? `执行 ${commandLabel}` : saving ? '正在保存更改'
      : item.status === 'running' ? `正在执行 ${commandLabel}` : `已执行 ${commandLabel}`;
  const output = [item.command?.stdout_preview, item.command?.stderr_preview].filter(Boolean).join('\n');
  const changes = approval?.changes ?? item.changes ?? [];
  const error = failed ? item.error?.message ?? item.detail : undefined;
  const submittedFeedback = refused ? approval?.answer?.feedback : undefined;
  const submit = async () => {
    if (!threadId || !item.runId || !approval || !pending || !decision || submitting) return;
    setSubmitting(true);
    const accepted = await answerCommandPermission(threadId, item.runId, approval.interactionId, item.callId, approval.commandHash,
      decision, decisionFeedback(decision, 'deny', feedback));
    if (!accepted) setSubmitting(false);
  };
  const Icon = item.status === 'running' && !pending && !refused ? Loader2 : SquareTerminal;
  return <div className="min-w-0 overflow-hidden rounded-lg">
    <button type="button" aria-expanded={expanded} aria-controls={id} onClick={() => setExpanded(value => !value)}
      className="timeline-disclosure-trigger ui-interactive grid min-h-8 w-full grid-cols-[16px_minmax(0,1fr)_16px] items-center gap-2 rounded-md px-1.5 py-1 text-left">
      <Icon className={cn('h-4 w-4', Icon === Loader2 && 'animate-spin motion-reduce:animate-none', failed || refused ? 'text-danger' : pending ? 'text-warning' : 'text-success')} strokeWidth={1.75} />
      <span className="flex min-w-0 items-center gap-2 text-[13px] font-normal text-text-900">
        <span className="min-w-0 truncate">{label}</span>{pending && <HumanInterventionBadge />}
      </span>
      <TimelineChevron open={expanded} />
    </button>
    <TimelineDisclosure id={id} open={expanded}>
      {expanded && <div className="timeline-detail-card pb-1.5">
        <article className={timelineDetailCardClassName}>
          <div className="flex min-h-[42px] items-center px-3.5 py-2">
            <code className="block min-w-0 truncate font-mono text-[11px] leading-[1.6] text-text-800">{firstLine}</code>
          </div>
          <CommandGroup label="命令输入"><pre className="m-0 max-h-[270px] overflow-auto whitespace-pre px-3.5 pb-3 font-mono text-[11px] leading-[1.7] text-text-700">{source}</pre></CommandGroup>
          {output && <CommandGroup label="命令输出"><pre className={cn("m-0 whitespace-pre-wrap break-words px-3.5 pb-3 font-mono text-[11px] leading-[1.75] text-text-600", failed && "text-danger")}>{output}</pre></CommandGroup>}
          {changes.length > 0 && <CommandGroup label="文件变更">{changes.map(target => <CommandFile key={target.diff?.filename ?? target.display_name} target={target} />)}</CommandGroup>}
          {pending && <DecisionControls options={[{ value: 'allow_once', label: '批准', tone: 'success' }, { value: 'deny', label: '拒绝', tone: 'danger' }]}
            selected={decision} onSelect={setDecision} onSubmit={() => void submit()} busy={submitting}
            feedback={feedback} onFeedback={decision === 'deny' ? setFeedback : undefined} label="命令变更审批" />}
          {(error || submittedFeedback) && <p className="m-0 whitespace-pre-wrap break-words border-t border-border px-3.5 py-3 text-xs leading-5 text-danger">{error ?? submittedFeedback}</p>}
        </article>
      </div>}
    </TimelineDisclosure>
  </div>;
}
