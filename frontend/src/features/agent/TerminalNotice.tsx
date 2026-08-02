import React from 'react';
import { AlertCircle, StopCircle } from 'lucide-react';
import { Disclosure } from '../../components/ui/primitives';
import type { TerminalNoticeItem } from './eventReducer';

export const TerminalNotice: React.FC<{ item: TerminalNoticeItem }> = ({ item }) => (
  <div className={item.status === 'failed'
    ? 'flex items-start gap-2 py-1.5 text-[13px] text-danger'
    : 'flex items-start gap-2 py-1.5 text-[13px] text-text-600'}>
    {item.status === 'failed'
      ? <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" strokeWidth={1.75} />
      : <StopCircle className="mt-0.5 h-4 w-4 shrink-0" strokeWidth={1.75} />}
    <div className="min-w-0">
      <p>{item.message}</p>
      {item.error?.retryable && <p className="mt-1 text-xs">可以调整后重新发起。</p>}
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
  </div>
);
