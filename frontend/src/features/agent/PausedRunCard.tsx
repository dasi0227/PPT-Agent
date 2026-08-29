import React, { useState } from 'react';
import { PauseCircle, Play } from 'lucide-react';
import { useRunStore } from '../../stores/runStore';
import { useActiveThreadId } from './useActiveSession';

export const PausedRunCard: React.FC<{ runId: string }> = ({ runId }) => {
  const threadId = useActiveThreadId();
  const resumeRun = useRunStore((state) => state.resumeRun);
  const [resuming, setResuming] = useState(false);

  const resume = async () => {
    if (!threadId || resuming) return;
    setResuming(true);
    const resumed = await resumeRun(threadId, runId);
    if (!resumed) setResuming(false);
  };

  return (
    <section
      className="mt-2 rounded-lg border border-border bg-surface p-3 shadow-[0_2px_7px_rgba(23,32,43,0.045)] motion-safe:animate-[timeline-enter_120ms_ease-out]"
      aria-labelledby={`paused-run-${runId}`}
    >
      <div className="grid grid-cols-[20px_minmax(0,1fr)] gap-2">
        <PauseCircle className="mt-0.5 h-[18px] w-[18px] text-text-600" strokeWidth={1.75} aria-hidden="true" />
        <div>
          <h3 id={`paused-run-${runId}`} className="text-[13px] font-semibold leading-5 text-text-900">
            任务已暂停
          </h3>
          <p className="mt-0.5 text-xs leading-5 text-text-600">
            服务中断后，此任务没有继续运行。你可以从中断处恢复，或直接发送新消息开始新的任务。
          </p>
        </div>
      </div>
      <div className="mt-2.5 flex justify-end">
        <button
          type="button"
          onClick={() => void resume()}
          disabled={!threadId || resuming}
          className="inline-flex h-7 items-center gap-1.5 rounded-md bg-accent px-2.5 text-xs font-semibold text-white hover:bg-accent/90 disabled:cursor-wait disabled:bg-text-400 disabled:opacity-60"
        >
          <Play className="h-3.5 w-3.5" strokeWidth={2} aria-hidden="true" />
          {resuming ? '正在恢复' : '继续任务'}
        </button>
      </div>
    </section>
  );
};
