import React, { useEffect, useMemo, useRef } from 'react';
import { AlertCircle, CheckCircle2, ShieldCheck } from 'lucide-react';
import { Disclosure, InlineNotice } from '../../components/ui/primitives';
import { useActiveSession } from './useActiveSession';
import { ArtifactCard } from './ArtifactCard';
import { FinalResultCard } from './FinalResultCard';
import { FinishBubble } from './FinishBubble';
import { MarkdownMessage } from './MarkdownMessage';
import { NeedsInputCard } from './NeedsInputCard';
import { PlanCard } from './PlanCard';
import { ThinkingBubble } from './ThinkingBubble';
import { ToolCallCard } from './ToolCallCard';
import type {
  ContextStatusItem,
  StrategyStatusItem,
  VerificationStatusItem,
} from './eventReducer';
import { strategyLabels, targetLabel } from './runtimeLabels';

const AGENT_CONTENT_TYPES = new Set([
  'markdown',
  'tool_call',
  'artifact',
  'final_result',
  'needs_input',
  'error',
  'context_status',
  'strategy_status',
  'verification_status',
]);

const ERROR_CODE_MESSAGES: Record<string, string> = {
  LLM_TIMEOUT: 'AI 响应超时，请稍后重试或降低任务复杂度。',
  LLM_BAD_REQUEST: 'AI 请求未能处理，请调整指令后重试。',
};

function primaryErrorMessage(message: string): string {
  return /[\u3400-\u9fff]/u.test(message)
    ? message
    : '运行未能完成，请查看错误详情后重试。';
}

function ExecutionMetaRow({
  context,
  strategy,
}: {
  context?: ContextStatusItem;
  strategy?: StrategyStatusItem;
}) {
  if (!context && !strategy) return null;
  return (
    <Disclosure label={[
      context ? '上下文已准备' : '',
      strategy ? strategyLabels[strategy.strategy] : '',
    ].filter(Boolean).join(' | ')}>
      <div className="space-y-1 text-xs text-text-600">
        {context && <p>上下文配置：{context.profile || '默认'}{context.readOnly ? '，只读' : ''}</p>}
        {strategy?.reason && <p>策略说明：{strategy.reason}</p>}
        {strategy && <p>风险：{strategy.risk || '未标注'}，复杂度：{strategy.complexity || '未标注'}</p>}
        {context?.warnings.map((warning) => <p key={warning} className="text-warning">{warning}</p>)}
      </div>
    </Disclosure>
  );
}

function VerificationSummary({ item }: { item: VerificationStatusItem }) {
  return (
    <div className="flex items-start gap-2 border-l-2 border-success px-3 py-1.5 text-xs text-text-600">
      <ShieldCheck className="mt-0.5 h-4 w-4 shrink-0 text-success" strokeWidth={1.75} />
      <div>
        <p className="font-medium text-text-900">验证完成</p>
        <p>{item.passedCount} 项通过{item.failedCount > 0 ? `，${item.failedCount} 项未通过` : ''}</p>
        {item.issues.length > 0 && (
          <Disclosure label={`查看 ${item.issues.length} 项问题`}>
            <ul className="space-y-1 text-danger">
              {item.issues.map((issue, index) => (
                <li key={`${issue.code ?? 'issue'}-${index}`}>{issue.message ?? issue.evidence ?? issue.code ?? '验证问题'}</li>
              ))}
            </ul>
          </Disclosure>
        )}
      </div>
    </div>
  );
}

function EmptyTimelineTitle() {
  return (
    <p className="text-center text-2xl font-bold italic tracking-tight text-text-400">Dasi PPT Agent</p>
  );
}

export const Timeline: React.FC = () => {
  const { timelineItems, status, plan } = useActiveSession();
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [timelineItems, plan]);

  const context = useMemo(
    () => [...timelineItems].reverse().find((item): item is ContextStatusItem => item.type === 'context_status'),
    [timelineItems],
  );
  const strategy = useMemo(
    () => [...timelineItems].reverse().find((item): item is StrategyStatusItem => item.type === 'strategy_status'),
    [timelineItems],
  );
  const visibleItems = timelineItems.filter((item) => item.type !== 'context_status' && item.type !== 'strategy_status');
  const hasUserTurn = timelineItems.some((item) => item.type === 'user_turn');
  const hasAgentContent = timelineItems.some((item) => AGENT_CONTENT_TYPES.has(item.type));
  const showThinking = (status === 'creating' || status === 'running') && hasUserTurn && !hasAgentContent;
  const runActive = status === 'creating' || status === 'running' || status === 'needs_input';
  const showEmptyWordmark = timelineItems.length === 0 && !plan && status === 'idle';

  return (
    <div className={showEmptyWordmark
      ? 'min-h-0 flex-1 overflow-hidden bg-panel p-3'
      : 'min-h-0 flex-1 space-y-3 overflow-y-auto bg-panel p-3'}>
      {showEmptyWordmark ? (
        <div className="flex h-full items-center justify-center overflow-hidden">
          <EmptyTimelineTitle />
        </div>
      ) : (
        <>
          <ExecutionMetaRow context={context} strategy={strategy} />
          {plan && <PlanCard plan={plan} running={runActive} />}

          {visibleItems.map((item) => {
        switch (item.type) {
          case 'user_turn':
            return (
              <div key={item.id} className="flex justify-end">
                <div className="max-w-[88%] rounded-lg border border-border bg-panel-muted px-3 py-2">
                  {item.target && (
                    <div className="mb-1 text-[10px] font-medium text-text-400">
                      {targetLabel(item.target.artifact as 'blueprint' | 'presentation', item.target.level as 'slide' | 'deck')}
                    </div>
                  )}
                  <MarkdownMessage content={item.text} />
                </div>
              </div>
            );
          case 'markdown':
            return <MarkdownMessage key={item.id} content={item.text} />;
          case 'tool_call':
            return <ToolCallCard key={item.id} item={item} />;
          case 'artifact':
            return <ArtifactCard key={item.id} item={item} />;
          case 'final_result':
            return typeof item.result === 'object' && item.result !== null
              ? <FinalResultCard key={item.id} item={item} />
              : <FinishBubble key={item.id} item={item} />;
          case 'needs_input':
            return <NeedsInputCard key={item.id} item={item} />;
          case 'error': {
            const friendly = item.code ? ERROR_CODE_MESSAGES[item.code] : undefined;
            const primaryMessage = friendly ?? primaryErrorMessage(item.message);
            return (
              <InlineNotice key={item.id} tone="danger">
                <div className="flex items-start gap-2">
                  <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
                  <div className="min-w-0">
                    <p>{primaryMessage}</p>
                    {item.retryable !== false && <p className="mt-1 text-xs">请检查输入或网络后重试。</p>}
                    {(item.code || item.requestId || item.technicalMessage || primaryMessage !== item.message) && (
                      <Disclosure label="错误详情">
                        <div className="space-y-1 font-mono text-[11px]">
                          {item.code && <p>错误码：{item.code}</p>}
                          {item.requestId && <p>请求 ID：{item.requestId}</p>}
                          {(item.technicalMessage || primaryMessage !== item.message) && (
                            <p>原始信息：{item.technicalMessage ?? item.message}</p>
                          )}
                        </div>
                      </Disclosure>
                    )}
                  </div>
                </div>
              </InlineNotice>
            );
          }
          case 'verification_status':
            return <VerificationSummary key={item.id} item={item} />;
          default:
            return null;
        }
          })}
          {showThinking && <ThinkingBubble />}
          {status === 'done' && visibleItems.length === 0 && (
            <div className="flex items-center gap-2 text-xs text-success">
              <CheckCircle2 className="h-4 w-4" /> 运行已完成
            </div>
          )}
        </>
      )}
      <div ref={bottomRef} />
    </div>
  );
};
