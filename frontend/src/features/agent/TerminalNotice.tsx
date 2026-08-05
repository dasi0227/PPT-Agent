import React from 'react';
import { StopCircle, XCircle } from 'lucide-react';
import { Disclosure } from '../../components/ui/primitives';
import type { TerminalNoticeItem } from './eventReducer';
import { useRunStore } from '../../stores/runStore';
import { useActiveThreadId } from './useActiveSession';

export const TerminalNotice: React.FC<{ item: TerminalNoticeItem }> = ({ item }) => {
  const threadId = useActiveThreadId();
  const retryRun = useRunStore((state) => state.retryRun);
  const canCreateRetry = item.status === 'failed'
    && (item.retryable ?? item.error?.retryable) === true;
  return <div className={item.status === 'failed'
    ? 'flex items-start gap-2 px-1.5 py-1.5 text-[13px] text-text-900'
    : 'flex items-start gap-2 px-1.5 py-1.5 text-[13px] text-text-600'}>
    {item.status === 'failed'
      ? <XCircle className="mt-0.5 h-4 w-4 shrink-0 text-danger" strokeWidth={1.75} />
      : <StopCircle className="mt-0.5 h-4 w-4 shrink-0" strokeWidth={1.75} />}
    <div className="min-w-0">
      <p>{item.message}</p>
      {canCreateRetry && (
        <button
          type="button"
          className="mt-1 text-xs font-medium text-accent hover:underline"
          onClick={() => threadId && void retryRun(threadId)}
        >
          重试（创建新任务）
        </button>
      )}
      {(item.error?.code || item.technicalMessage || item.requestId) && (
        <Disclosure label="错误详情">
          <div className="space-y-1 font-mono text-[11px]">
            {item.error?.code && <p>错误码：{item.error.code}</p>}
            {item.requestId && <p>请求 ID：{item.requestId}</p>}
            {item.technicalMessage && <p>{item.technicalMessage}</p>}
          </div>
        </Disclosure>
      )}
    </div>
  </div>;
};
