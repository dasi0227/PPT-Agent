import React, { useState } from 'react';
import { ChevronDown, ChevronRight, BrainCircuit } from 'lucide-react';
import { ThoughtItem } from './eventReducer';
import { MarkdownMessage } from './MarkdownMessage';

export const ThoughtCard: React.FC<{ item: ThoughtItem }> = ({ item }) => {
  const [expanded, setExpanded] = useState(false);

  return (
    <div className="border border-border rounded-md bg-surface overflow-hidden my-2">
      <button 
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center px-3 py-2 text-sm text-text-600 hover:bg-black/5 transition-colors"
      >
        {expanded ? <ChevronDown className="w-4 h-4 mr-1" /> : <ChevronRight className="w-4 h-4 mr-1" />}
        <BrainCircuit className="w-4 h-4 mr-2" />
        <span>执行思路</span>
      </button>
      {expanded && (
        <div className="px-4 py-3 border-t border-border text-sm text-text-600">
          <MarkdownMessage content={item.text} />
        </div>
      )}
    </div>
  );
};
