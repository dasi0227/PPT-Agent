import React from 'react';
import { CheckCircle2 } from 'lucide-react';
import { FinalResultItem } from './eventReducer';
import { MarkdownMessage } from './MarkdownMessage';

export const FinishBubble: React.FC<{ item: FinalResultItem }> = ({ item }) => {
  const content = typeof item.result === 'string' 
    ? item.result 
    : (item.result?.summary || '已完成');

  return (
    <div className="flex justify-start my-2">
      <div className="max-w-[85%] bg-surface border border-border rounded-lg p-3 shadow-sm relative">
        <MarkdownMessage content={content} />
        <div className="flex items-center justify-end mt-2 pt-2 border-t border-border/50">
          <CheckCircle2 className="w-3 h-3 text-mode-final mr-1" />
          <span className="text-[10px] text-mode-final font-medium">已完成</span>
        </div>
      </div>
    </div>
  );
};
