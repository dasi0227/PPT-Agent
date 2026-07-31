import React, { useEffect, useRef } from 'react';
import { useActiveSession } from './useActiveSession';
import { MarkdownMessage } from './MarkdownMessage';
import { ThoughtCard } from './ThoughtCard';
import { ToolCallCard } from './ToolCallCard';
import { PlanCard } from './PlanCard';
import { ArtifactCard } from './ArtifactCard';
import { FinalResultCard } from './FinalResultCard';
import { FinishBubble } from './FinishBubble';
import { NeedsInputCard } from './NeedsInputCard';
import { ThinkingBubble } from './ThinkingBubble';

// AGENT_CONTENT_TYPES：一旦 timeline 出现任何"agent 类"内容，就撤下 ThinkingBubble；
// tool_call / artifact 也算已有反馈（用户能看到 agent 在做事），一并进白名单。
const AGENT_CONTENT_TYPES = new Set([
  'markdown',
  'thought',
  'tool_call',
  'artifact',
  'final_result',
  'needs_input',
  'error',
  'context_status',
]);

// ERROR_CODE_MESSAGES：错误码到中文友好文案的映射。缺省仍回落 item.message，
// 避免在未覆盖的错误码上出现"空提示"。
const ERROR_CODE_MESSAGES: Record<string, string> = {
  LLM_TIMEOUT: 'AI 响应超时，请稍后重试或调低复杂度',
  LLM_BAD_REQUEST: 'AI 请求失败',
};

export const Timeline: React.FC = () => {
  const { timelineItems, status, plan } = useActiveSession();
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [timelineItems, plan]);

  // 思考气泡显示条件：status=running，且用户已发过 user_turn，且尚无任何 agent 内容。
  const hasUserTurn = timelineItems.some((it) => it.type === 'user_turn');
  const hasAgentContent = timelineItems.some((it) => AGENT_CONTENT_TYPES.has(it.type));
  const showThinking = status === 'running' && hasUserTurn && !hasAgentContent;

  return (
    <div className="flex-1 overflow-y-auto p-4 space-y-4">
      {timelineItems.length === 0 && !plan && status === 'idle' ? (
        <div className="text-center text-text-400 text-sm mt-10">
          How can I help you with this presentation?
        </div>
      ) : null}

      {/* plan 与 progress 正交：plan 是步骤清单，常驻 timeline 顶部；progress 在面板头部 */}
      {plan && <PlanCard plan={plan} />}

      {timelineItems.map((item) => {
        switch (item.type) {
          case 'user_turn':
            return (
              <div key={item.id} className="flex justify-end">
                <div className="max-w-[85%] rounded-lg bg-black/5 border border-border px-3 py-2">
                  {item.target && (
                    <div className="mb-1 text-[10px] font-medium uppercase tracking-wide text-text-400">
                      {item.target.artifact} / {item.target.level} · {item.interaction?.intent}
                    </div>
                  )}
                  <MarkdownMessage content={item.text} />
                </div>
              </div>
            );
          case 'markdown':
            return <MarkdownMessage key={item.id} content={item.text} />;
          case 'thought':
            return <ThoughtCard key={item.id} item={item} />;
          case 'tool_call':
            if (item.hiddenFromTimeline) return null;
            return <ToolCallCard key={item.id} item={item} />;
          case 'artifact':
            return <ArtifactCard key={item.id} item={item} />;
          case 'final_result':
            if (typeof item.result === 'object' && (typeof item.result.slide_count === 'number' || typeof item.result.artifact === 'string')) {
              return <FinalResultCard key={item.id} item={item} />;
            }
            return <FinishBubble key={item.id} item={item} />;
          case 'needs_input':
            return <NeedsInputCard key={item.id} item={item} />;
          case 'error':
            {
              const friendly = item.code ? ERROR_CODE_MESSAGES[item.code] : undefined;
              return (
                <div key={item.id} className="p-3 bg-mode-error/10 border border-mode-error/20 text-mode-error text-sm rounded-md">
                  <strong>Error{item.code ? ` [${item.code}]` : ''}:</strong>{' '}
                  {friendly ?? item.message}
                </div>
              );
            }
          case 'context_status':
            return (
              <div key={item.id} className="rounded-md border border-border bg-black/[0.02] px-3 py-2 text-xs text-text-400">
                Context ready · {item.profile}{item.readOnly ? ' · read-only' : ''}
                {item.warnings.length > 0 && (
                  <div className="mt-1 text-amber-600">{item.warnings.join(' · ')}</div>
                )}
              </div>
            );
          default:
            return null;
        }
      })}
      {showThinking && <ThinkingBubble />}
      <div ref={bottomRef} />
    </div>
  );
};
