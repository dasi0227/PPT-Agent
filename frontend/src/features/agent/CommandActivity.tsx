import { useEffect, useId, useRef, useState, type ReactNode } from 'react';
import {
  Check,
  Clipboard,
  Gauge,
  GitCommitHorizontal,
  Handshake,
  MessageSquarePlus,
  RefreshCw,
  Signature,
  Sparkles,
  SportShoe,
  X,
} from 'lucide-react';
import type { CommandStatus } from '../../stores/commandRuntime';
import { showGlobalError, showGlobalSuccess } from '../../stores/toastStore';
import { cn } from '../../lib/utils';
import { formatTimestamp } from '../../lib/formatTimestamp';
import { MarkdownMessage } from './MarkdownMessage';
import { LongContent } from './LongContent';
import { TimelineDisclosure } from './TimelineDisclosure';
export type CommandKind = 'kickoff' | 'handoff' | 'commit' | 'compact' | 'rename' | 'polish';
const icons = {
  kickoff: SportShoe,
  handoff: Handshake,
  commit: GitCommitHorizontal,
  compact: Gauge,
  rename: Signature,
  polish: Sparkles,
};
export const commandSteps: Record<CommandKind, [string, string, string]> = {
  kickoff: ['读取项目', '组织目标', '生成简报'],
  handoff: ['梳理进展', '提炼待办', '生成交接'],
  commit: ['整理变更', '生成说明', '写入版本'],
  compact: ['读取上下文', '提炼摘要', '更新上下文'],
  rename: ['理解会话', '生成名称', '更新名称'],
  polish: ['理解意图', '优化表达', '生成建议'],
};
const loadingTitles: Record<CommandKind, string> = {
  kickoff: '正在生成启动简报',
  handoff: '正在整理交接简报',
  commit: '正在提交项目版本',
  compact: '正在压缩上下文',
  rename: '正在生成会话名称',
  polish: '正在润色当前输入',
};
const failureTitles: Record<CommandKind, string> = {
  kickoff: '启动简报生成失败',
  handoff: '交接简报生成失败',
  commit: '项目版本提交失败',
  compact: '上下文压缩失败',
  rename: '会话名称生成失败',
  polish: '输入内容润色失败',
};
const labels: Record<CommandKind, string> = {
  kickoff: '启动简报',
  handoff: '交接简报',
  commit: '项目版本',
  compact: '上下文',
  rename: '会话名称',
  polish: '输入内容',
};
export function CommandActivity({
  kind,
  title,
  timestamp,
  status = 'completed',
  phase = -1,
  cancellable = true,
  metadata,
  content,
  children,
  onCancel,
  onRetry,
  onRevise,
  onPrimary,
  primaryLabel,
  copyText,
  busy = false,
  defaultOpen = false,
}: {
  kind: CommandKind;
  title: string;
  timestamp: number;
  status?: CommandStatus;
  phase?: number;
  cancellable?: boolean;
  metadata?: ReactNode;
  content?: string;
  children?: ReactNode;
  onCancel?: () => void;
  onRetry?: () => void;
  onRevise?: (feedback: string) => Promise<boolean>;
  onPrimary?: () => void | Promise<void>;
  primaryLabel?: string;
  copyText?: string;
  busy?: boolean;
  defaultOpen?: boolean;
}) {
  const [open, setOpen] = useState(defaultOpen),
    [feedback, setFeedback] = useState(''),
    [acting, setActing] = useState(false);
  const previous = useRef(status),
    id = useId(),
    Icon = icons[kind];
  useEffect(() => {
    if (previous.current === 'loading' && status === 'completed')
      setOpen(Boolean(onRevise) || kind === 'rename');
    previous.current = status;
  }, [status, kind, onRevise]);
  const ready = status === 'completed',
    running = status === 'loading';
  const displayTitle = running
    ? loadingTitles[kind]
    : status === 'failed'
      ? failureTitles[kind]
      : status === 'canceled'
        ? `${labels[kind]}已停止`
        : title;
  const revise = async () => {
    if (!onRevise || acting || busy) return;
    setActing(true);
    try {
      if (await onRevise(feedback.trim())) setFeedback('');
    } finally {
      setActing(false);
    }
  };
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(copyText ?? '');
      showGlobalSuccess('已复制');
    } catch {
      showGlobalError('复制失败，请选中文本手动复制');
    }
  };
  const primary = async () => {
    if (!onPrimary || acting || busy) return;
    setActing(true);
    try {
      await onPrimary();
    } catch {
      showGlobalError('操作失败，请重试');
    } finally {
      setActing(false);
    }
  };
  return (
    <article className="command-activity" aria-busy={running}>
      <div className="command-activity-head">
        <button
          type="button"
          disabled={!ready}
          aria-label={`${kind}: ${displayTitle}`}
          aria-expanded={ready && open}
          aria-controls={id}
          onClick={() => setOpen((value) => !value)}
          className="command-activity-summary"
          title={ready ? `${title} · 点击${open ? '收起' : '展开'}` : displayTitle}
        >
          <span
            className={cn(
              'command-activity-icon',
              status === 'failed'
                ? 'bg-danger-soft text-danger'
                : status === 'canceled'
                  ? 'bg-panel-muted text-text-600'
                  : 'bg-success-soft text-success',
            )}
          >
            <Icon size={18} strokeWidth={1.75} aria-hidden="true" />
            <span className="command-activity-name">{kind}</span>
          </span>
          <span className="command-activity-title">
            <span className="truncate font-normal">{displayTitle}</span>
          </span>
          <span className="command-activity-meta">
            <time dateTime={new Date(timestamp).toISOString()}>
              {formatTimestamp(timestamp)}
            </time>
            {metadata && (
              <>
                <span aria-hidden="true">·</span>
                {metadata}
              </>
            )}
          </span>
        </button>
        {running && cancellable && onCancel && (
          <button className="command-action command-stop" onClick={onCancel}>
            <X size={13} />
            停止
          </button>
        )}
        {(status === 'failed' || status === 'canceled') && onRetry && (
          <button className="command-action command-retry" disabled={busy} onClick={onRetry}>
            <RefreshCw size={13} />
            重试
          </button>
        )}
      </div>
      {running && (
        <div className="command-progress" role="group" aria-label={`${kind}进度`}>
          <span className="command-progress-line" aria-hidden="true">
            <span style={{ width: `${Math.max(0, phase) * 50}%` }} />
          </span>
          {commandSteps[kind].map((label, index) => (
            <span
              key={label}
              className={cn(
                'command-step',
                index < phase && 'is-complete',
                index === phase && 'is-current',
              )}
              aria-current={index === phase ? 'step' : undefined}
            >
              <span className="command-step-node" aria-hidden="true" />
              {label}
            </span>
          ))}
        </div>
      )}
      <TimelineDisclosure open={ready && open}>
        {ready && open && (
          <div id={id} className="timeline-detail-card command-detail-card">
            <LongContent
              contentClassName="text-[13px] leading-[1.85] text-text-700"
              testId="command-content-preview"
            >
              {content && <MarkdownMessage content={content} />} {children}
            </LongContent>
            {(onRevise || onPrimary || copyText) && (
              <div className="command-footer">
                {onRevise && (
                  <>
                    <form
                      id={`${id}-retry`}
                      onSubmit={(event) => {
                        event.preventDefault();
                        void revise();
                      }}
                      className="command-feedback"
                    >
                      <input
                        aria-label="调整建议，按 Enter 重试"
                        placeholder="调整建议（可选），Enter 重试"
                        value={feedback}
                        onChange={(event) => setFeedback(event.target.value)}
                        maxLength={4000}
                        disabled={acting || busy}
                        onKeyDown={(event) => {
                          if (event.key === 'Enter' && event.nativeEvent.isComposing)
                            event.preventDefault();
                        }}
                      />
                    </form>
                    <button
                      type="submit"
                      form={`${id}-retry`}
                      className="command-action command-retry"
                      disabled={acting || busy}
                    >
                      <RefreshCw size={13} />
                      重试
                    </button>
                  </>
                )}
                {copyText && (
                  <button className="command-action command-copy" onClick={() => void copy()}>
                    <Clipboard size={13} />
                    复制
                  </button>
                )}
                {onPrimary && (
                  <button
                    className="command-action command-primary"
                    disabled={acting || busy}
                    onClick={() => void primary()}
                  >
                    {kind === 'polish' ? <Check size={13} /> : <MessageSquarePlus size={13} />}{' '}
                    {primaryLabel}
                  </button>
                )}
              </div>
            )}
          </div>
        )}
      </TimelineDisclosure>
    </article>
  );
}
