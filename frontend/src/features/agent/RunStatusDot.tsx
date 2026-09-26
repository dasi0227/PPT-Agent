import type { RunStatus } from '../../stores/runStore';

const STATUS_STYLE: Partial<Record<RunStatus, { label: string; className: string }>> = {
  running: { label: '运行中', className: 'bg-accent animate-pulse motion-reduce:animate-none' },
  waiting: { label: '等待输入', className: 'bg-warning animate-pulse motion-reduce:animate-none' },
  paused: { label: '已暂停', className: 'bg-text-400' },
};

export function RunStatusDot({ status }: { status: RunStatus }) {
  const style = STATUS_STYLE[status];
  if (!style) return null;

  return (
    <span
      aria-label={style.label}
      title={style.label}
      className={`h-2 w-2 shrink-0 rounded-full ${style.className}`}
    />
  );
}
