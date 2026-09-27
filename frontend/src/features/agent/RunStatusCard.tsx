import { Play } from 'lucide-react';
import { interactionCardClassName, interactionReasonClassName, interactionTitleClassName } from './interactionCardStyles';

export function RunStatusCard({ status, message, onContinue, continuing = false }: {
  status: 'failed' | 'error' | 'canceled';
  message: string;
  onContinue?: () => void;
  continuing?: boolean;
}) {
  const title = status === 'failed' ? '任务执行失败' : status === 'error' ? '任务执行异常' : '任务执行中断';
  return <article className={`${interactionCardClassName} px-4 pb-4 pt-[15px]`}>
    <header className="flex flex-wrap items-center justify-between gap-x-3 gap-y-2">
      <h3 className={interactionTitleClassName}>{title}</h3>
      {onContinue && <button type="button" disabled={continuing} onClick={onContinue}
        className="ui-primary inline-flex min-h-[31px] shrink-0 items-center justify-center gap-[5px] rounded-md px-[11px] py-1 text-xs font-semibold focus-visible:underline focus-visible:underline-offset-[3px] disabled:cursor-wait disabled:opacity-50">
        <Play className="h-3.5 w-3.5" strokeWidth={1.75} aria-hidden="true" />{continuing ? '正在继续' : '继续执行'}
      </button>}
    </header>
    <p className={interactionReasonClassName}>{status === 'canceled' ? '你已停止本次任务。' : message}</p>
  </article>;
}
