import { ChevronRight, GitCommitHorizontal, RotateCw } from 'lucide-react';
import { useState } from 'react';
import type { GitCommitPhase } from '../../api/types';
import { cn } from '../../lib/utils';
import { useComposerStore } from '../../stores/composerStore';
import { useGitCommitStore } from '../../stores/gitCommitStore';
import { useProjectStore } from '../../stores/projectStore';
import { useThreadStore } from '../../stores/threadStore';
import type { GitCommitTimelineItem } from './eventReducer';
import { TimelineDisclosure } from './TimelineDisclosure';

const phases: Array<{ id: GitCommitPhase; label: string }> = [
  { id: 'staging', label: '整理变更' },
  { id: 'analyzing', label: '生成说明' },
  { id: 'committing', label: '写入版本' },
];

function formatDate(timestamp: number): string {
  const date = new Date(timestamp);
  const pad = (value: number) => String(value).padStart(2, '0');
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

export function GitCommitProgress({ phase }: { phase: GitCommitPhase | null }) {
  const activeIndex = Math.max(0, phases.findIndex((item) => item.id === phase));
  const fill = activeIndex === 0 ? '0px' : activeIndex === 1 ? 'calc(50% - 20px)' : 'calc(100% - 40px)';
  return (
    <div className="px-1.5 py-2" role="status" aria-label={`提交项目版本：${phases[activeIndex].label}`}>
      <div className="relative flex w-full justify-between">
        <span className="absolute left-5 right-5 top-[5px] h-px bg-border" aria-hidden="true" />
        <span
          className="absolute left-5 top-[5px] h-px bg-success transition-[width] duration-200"
          style={{ width: fill }}
          aria-hidden="true"
        />
        {phases.map((item, index) => {
          const completed = index < activeIndex;
          const active = index === activeIndex;
          return (
            <span key={item.id} className="relative z-[1] w-10 flex-none pt-4 text-center text-[10px] text-text-400">
              <span
                className={cn(
                  'absolute left-1/2 top-0 h-[11px] w-[11px] -translate-x-1/2 rounded-full border-[1.5px] bg-panel',
                  completed && 'border-success bg-success',
                  active && 'border-success',
                  !completed && !active && 'border-border',
                )}
                aria-hidden="true"
              />
              {item.label}
            </span>
          );
        })}
      </div>
    </div>
  );
}

function Divider() {
  return <span className="h-2.5 w-px shrink-0 bg-border" aria-hidden="true" />;
}

export function GitCommitEvent({ item }: { item: GitCommitTimelineItem }) {
  const [expanded, setExpanded] = useState(false);
  if (item.status === 'failed') return <GitCommitFailure item={item} />;
  return (
    <div className="overflow-hidden">
      <button
        type="button"
        aria-expanded={expanded}
        onClick={() => setExpanded((value) => !value)}
        className="grid min-h-10 w-full grid-cols-[26px_minmax(0,1fr)_14px] items-start gap-2 px-1 py-1.5 text-left"
      >
        <span className="grid h-[26px] w-[26px] place-items-center rounded-full border border-success/35 bg-success-soft text-success">
          <GitCommitHorizontal className="h-3.5 w-3.5" strokeWidth={1.8} />
        </span>
        <span className="min-w-0">
          <span className="block truncate text-xs font-semibold text-text-900">{item.title}</span>
          <span className="mt-0.5 flex min-w-0 flex-wrap items-center gap-x-1.5 gap-y-0.5 font-mono text-[10px] text-text-400">
            <span>{formatDate(item.timestamp)}</span>
            <Divider />
            <span className="inline-flex items-center gap-1">
              <GitCommitHorizontal className="h-2.5 w-2.5" strokeWidth={1.8} />
              {item.branch}
            </span>
            <span>{item.hash}</span>
            <Divider />
            <span>{item.filesChanged} files</span>
            <span className="text-success">+{item.insertions}</span>
            <span className="text-danger">-{item.deletions}</span>
          </span>
        </span>
        <ChevronRight
          className={cn('mt-1 h-3.5 w-3.5 text-text-400 transition-transform', expanded && 'rotate-90')}
          strokeWidth={1.75}
        />
      </button>
      <TimelineDisclosure open={expanded}>
        {expanded && (
          <ul className="ml-[34px] space-y-1 border-t border-border py-2 pr-2 text-[11px] leading-5 text-text-600">
            {(item.items ?? []).map((entry) => (
              <li key={entry} className="flex gap-2">
                <span className="mt-2 h-1 w-1 shrink-0 rounded-full bg-success" />
                <span>{entry}</span>
              </li>
            ))}
          </ul>
        )}
      </TimelineDisclosure>
    </div>
  );
}

function GitCommitFailure({ item }: { item: GitCommitTimelineItem }) {
  const activeProjectId = useProjectStore((state) => state.activeProjectId);
  const activeThreadIdByProjectId = useThreadStore((state) => state.activeThreadIdByProjectId);
  const model = useComposerStore((state) => state.modelProfileName);
  const session = useGitCommitStore((state) => (
    activeProjectId ? state.sessions[activeProjectId] : undefined
  ));
  const start = useGitCommitStore((state) => state.start);
  const busy = session?.status === 'creating' || session?.status === 'running';
  const retry = () => {
    if (!activeProjectId || !model) return;
    const threadId = activeThreadIdByProjectId[activeProjectId];
    if (!threadId) return;
    void start(activeProjectId, threadId, model);
  };
  return (
    <div className="grid min-h-10 grid-cols-[26px_minmax(0,1fr)_28px] items-start gap-2 px-1 py-1.5">
      <span className="grid h-[26px] w-[26px] place-items-center rounded-full border border-danger/35 bg-danger-soft text-danger">
        <GitCommitHorizontal className="h-3.5 w-3.5" strokeWidth={1.8} />
      </span>
      <span className="min-w-0">
        <span className="block text-xs font-semibold text-text-900">项目版本提交失败</span>
        <span className="mt-0.5 flex min-w-0 flex-wrap items-center gap-1.5 font-mono text-[10px] text-text-400">
          <span>{formatDate(item.timestamp)}</span>
          <Divider />
          <span>提交失败，请重新尝试或手动提交</span>
        </span>
      </span>
      <button
        type="button"
        onClick={retry}
        disabled={busy || !activeProjectId || !model}
        className="grid h-7 w-7 place-items-center rounded-md text-text-600 hover:bg-danger-soft hover:text-danger disabled:cursor-not-allowed disabled:opacity-40"
        aria-label="重新提交"
        title="重新提交"
      >
        <RotateCw className="h-3.5 w-3.5" strokeWidth={1.75} />
      </button>
    </div>
  );
}
