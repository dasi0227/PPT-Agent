import { useState } from 'react';
import { runsApi } from '../../api/runs';
import { useDeckStore } from '../../stores/deckStore';
import { useProjectStore } from '../../stores/projectStore';
import { resourceApprovalKey, useResourceApprovalStore } from '../../stores/resourceApprovalStore';
import type { ToolActivityItem } from './eventReducer';
import { DecisionControls, decisionFeedback } from './HumanIntervention';

export function ResourceApprovalControls({ item }: { item: ToolActivityItem }) {
  const projectId = useProjectStore(state => state.activeProjectId);
  const key = resourceApprovalKey(item.runId ?? '', item.approval?.interactionId ?? '');
  const flow = useResourceApprovalStore(state => state.editStates[key]);
  const active = useResourceApprovalStore(state => state.active);
  const activeDocument = useDeckStore(state => state.activeDocument);
  const [decision, setDecision] = useState<'approve' | 'manual' | 'reject' | ''>('');
  const [feedback, setFeedback] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const editing = active && resourceApprovalKey(active.runId, active.interactionId) === key
    && activeDocument === item.approval?.resource && flow?.phase === 'editing';
  const blocked = Boolean(flow?.busy || (decision !== 'reject' && (flow?.dirty || (decision === 'manual' && editing))));
  const submit = async () => {
    if (!item.runId || !item.approval || item.approval.answer || !decision || busy || blocked) return;
    if (decision === 'manual' && flow?.phase !== 'review') {
      if (!projectId) return;
      useResourceApprovalStore.getState().open({ projectId, runId: item.runId, interactionId: item.approval.interactionId,
        callId: item.callId, resource: item.approval.resource });
      useDeckStore.getState().setActiveDocument(item.approval.resource);
      return;
    }
    setBusy(true); setError('');
    try {
      const current = await runsApi.getResourceEditApproval(item.runId, item.approval.interactionId);
      if (current.state !== 'pending') throw new Error('审批已处理，请刷新');
      if (current.revision !== item.approval.revision || (decision === 'manual' && current.revision !== flow?.revision)) {
        throw new Error('草稿已更新，请核对最新内容后重试');
      }
      await runsApi.decideResourceEditApproval(item.runId, current.interaction_id, item.callId, current.revision,
        decision === 'reject' ? 'reject' : 'approve', decisionFeedback(decision, 'reject', feedback));
      useResourceApprovalStore.getState().finish(key);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '审批失败'); setBusy(false);
    }
  };
  return <DecisionControls options={[
    { value: 'approve', label: '批准', tone: 'success' }, { value: 'manual', label: '手动编辑', tone: 'warning' },
    { value: 'reject', label: '拒绝', tone: 'danger' },
  ]} selected={decision} onSelect={setDecision} onSubmit={() => void submit()} busy={busy} disabled={blocked}
    feedback={feedback} onFeedback={decision === 'reject' ? setFeedback : undefined} error={error} label="资源编辑审批" inset />;
}
