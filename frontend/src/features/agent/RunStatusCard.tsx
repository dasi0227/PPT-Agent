import { Play } from 'lucide-react';
import { MarkdownMessage } from './MarkdownMessage';

export function RunStatusCard({ message, onContinue, continuing = false }: {
  message: string;
  onContinue?: () => void;
  continuing?: boolean;
}) {
  return <article className="pb-4 pt-2 text-sm leading-[1.65] text-text-900">
    <MarkdownMessage content={message} className="[&>p:first-child]:mt-0 [&>p:last-child]:mb-0" />
    {onContinue && <button type="button" disabled={continuing} onClick={onContinue}
      className="ui-interactive mt-2 inline-flex min-h-8 items-center gap-1.5 rounded-md px-2 text-xs text-text-600 focus-visible:underline disabled:cursor-wait disabled:opacity-50">
      <Play className="h-3.5 w-3.5" strokeWidth={1.75} aria-hidden="true" />{continuing ? '正在继续' : '继续执行'}
    </button>}
  </article>;
}
