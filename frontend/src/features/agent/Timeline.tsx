import React, { useEffect, useRef } from 'react';
import { useActiveSession } from './useActiveSession';
import { MarkdownMessage } from './MarkdownMessage';
import { ThoughtCard } from './ThoughtCard';
import { ToolCallCard } from './ToolCallCard';
import { PlanCard } from './PlanCard';
import { ArtifactCard } from './ArtifactCard';
import { FinalResultCard } from './FinalResultCard';
import { NeedsInputCard } from './NeedsInputCard';

export const Timeline: React.FC = () => {
  const { timelineItems, status } = useActiveSession();
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [timelineItems]);

  return (
    <div className="flex-1 overflow-y-auto p-4 space-y-4">
      {timelineItems.length === 0 && status === 'idle' ? (
        <div className="text-center text-text-400 text-sm mt-10">
          How can I help you with this presentation?
        </div>
      ) : null}

      {timelineItems.map((item) => {
        switch (item.type) {
          case 'markdown':
            return <MarkdownMessage key={item.id} content={item.text} />;
          case 'thought':
            return <ThoughtCard key={item.id} item={item} />;
          case 'tool_call':
            return <ToolCallCard key={item.id} item={item} />;
          case 'plan':
            return <PlanCard key={item.id} item={item} />;
          case 'artifact':
            return <ArtifactCard key={item.id} item={item} />;
          case 'final_result':
            return <FinalResultCard key={item.id} item={item} />;
          case 'needs_input':
            return <NeedsInputCard key={item.id} item={item} />;
          case 'error':
            return (
              <div key={item.id} className="p-3 bg-mode-error/10 border border-mode-error/20 text-mode-error text-sm rounded-md">
                <strong>Error{item.code ? ` [${item.code}]` : ''}:</strong> {item.message}
              </div>
            );
          default:
            return null;
        }
      })}
      <div ref={bottomRef} />
    </div>
  );
};
