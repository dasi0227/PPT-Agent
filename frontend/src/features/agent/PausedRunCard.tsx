import { useEffect, useState } from 'react';
import { runsApi } from '../../api/runs';
import { RunStatusCard } from './RunStatusCard';

const reasons: Record<string, string> = {
  server_shutdown: '服务已关闭，本次任务未能完成。',
  server_restarted: '服务已重启，本次任务未能完成。',
  journal_write_failed: '运行记录保存失败，本次任务已停止。',
  transcript_write_failed: '对话记录保存失败，本次任务已停止。',
  event_protocol_invalid: '运行事件异常，本次任务已停止。',
  resume_event_failed: '继续执行时发生异常，本次任务未能恢复。',
};

// Paused remains an internal lifecycle state; the user sees an execution exception.
export function PausedRunCard({ runId }: { runId: string }) {
  const [message, setMessage] = useState('服务中断，本次任务未能完成。');
  useEffect(() => {
    let disposed = false;
    void runsApi.get(runId).then(run => {
      if (!disposed) setMessage(reasons[run.pause_reason ?? ''] ?? '服务中断，本次任务未能完成。');
    }).catch(() => {});
    return () => { disposed = true; };
  }, [runId]);
  return <RunStatusCard status="error" message={message} />;
}
