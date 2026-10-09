import { interactionCardClassName, interactionReasonClassName, interactionInsetClassName } from './interactionCardStyles';
import { ShieldCheck } from 'lucide-react';
import { useState } from 'react';
import { useRunStore } from '../../stores/runStore';
import type { CommandPermissionItem } from './eventReducer';
import { useActiveThreadId } from './useActiveSession';
import { HumanIntervention, DecisionControls, SubmittedFeedback, decisionFeedback } from './HumanIntervention';

export function CommandPermissionCard({ item }: { item: CommandPermissionItem }) {
  const threadId = useActiveThreadId();
  const answerCommandPermission = useRunStore(state => state.answerCommandPermission);
  const [decision, setDecision] = useState<'allow_once' | 'deny' | ''>('');
  const [feedback, setFeedback] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const submit = async () => {
    if (!threadId || !item.runId || !decision || submitting || item.answer) return;
    setSubmitting(true);
    const accepted = await answerCommandPermission(threadId, item.runId, item.interactionId, item.callId, item.commandHash,
      decision, decisionFeedback(decision, 'deny', feedback));
    if (!accepted) setSubmitting(false);
  };
  return <HumanIntervention id={`command-permission-details-${item.interactionId}`} pending={!item.answer} icon={ShieldCheck}
    label={item.answer ? item.answer.decision === 'allow_once' ? '已允许命令执行' : '已拒绝命令执行' : '授权执行命令'}
    tone={item.answer?.decision === 'deny' ? 'danger' : 'success'}>
    <article className={`${interactionCardClassName} timeline-detail-card overflow-hidden`}>
      <div className="px-4 pb-4 pt-[15px]">
        <p className={interactionReasonClassName}>{item.reason}</p>
        <code className={`${interactionInsetClassName} block overflow-x-auto whitespace-pre-wrap break-words font-mono text-[11px] leading-[1.55] text-text-800`}>{item.command}</code>
      </div>
      {item.answer ? <SubmittedFeedback feedback={item.answer.feedback} /> : <DecisionControls
        options={[{ value: 'allow_once', label: '批准', tone: 'success' }, { value: 'deny', label: '拒绝', tone: 'danger' }]}
        selected={decision} onSelect={setDecision} onSubmit={() => void submit()} busy={submitting}
        feedback={feedback} onFeedback={decision === 'deny' ? setFeedback : undefined} label="命令授权操作" />}
    </article>
  </HumanIntervention>;
}
