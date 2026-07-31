import React from 'react';
import { FinalResultItem } from './eventReducer';
import { MarkdownMessage } from './MarkdownMessage';

export const FinishBubble: React.FC<{ item: FinalResultItem }> = ({ item }) => {
  const content = typeof item.result === 'string'
    ? item.result
    : (item.result?.summary || '已完成');

  return (
    <div className="flex justify-start my-4">
      <div className="max-w-[85%] bg-surface border border-border rounded-lg p-3 shadow-sm text-sm text-text-900 break-words">
        <MarkdownMessage content={content} />
      </div>
    </div>
  );
};
