import { useEffect, useState } from 'react';
import { runsApi } from '../../api/runs';
import type { TerminalNoticeItem } from './eventReducer';
import { useRunStore } from '../../stores/runStore';
import { useActiveSession, useActiveThreadId } from './useActiveSession';
import { RunStatusCard } from './RunStatusCard';
import { FinalChangeSummary } from './FinalMessage';

export function TerminalNotice({ item }: { item: TerminalNoticeItem }) {
  const threadId = useActiveThreadId();
  const session = useActiveSession();
  const resumeRun = useRunStore(state => state.resumeRun);
  const [canContinue, setCanContinue] = useState(false);
  const [continuing, setContinuing] = useState(false);
  const busy = ['creating', 'running', 'waiting', 'recovering', 'canceling', 'paused'].includes(session.status);
  const status = item.reason === 'superseded' ? 'error' : item.status;
  useEffect(() => {
    let disposed = false;
    setCanContinue(false);
    if (!item.runId || status === 'error' || busy) return;
    void runsApi.get(item.runId).then(run => {
      if (!disposed) setCanContinue(run.can_continue === true);
    }).catch(() => { /* A stale or unavailable run must not expose a continuation action. */ });
    return () => { disposed = true; };
  }, [item.runId, item.timestamp, status, busy, session.activeRunId]);
  const resume = async () => {
    if (!threadId || !item.runId || continuing) return;
    setContinuing(true);
    const resumed = await resumeRun(threadId, item.runId);
    if (!resumed) { setCanContinue(false); setContinuing(false); }
  };
  return <>
    {item.affectedTargets.length > 0 && <FinalChangeSummary targets={item.affectedTargets} />}
    <RunStatusCard status={status} message={item.reason === 'superseded' ? '此前任务因服务中断而结束。' : item.message}
    continuing={continuing} onContinue={continuing || (canContinue && !busy) ? () => void resume() : undefined} />
  </>;
}
