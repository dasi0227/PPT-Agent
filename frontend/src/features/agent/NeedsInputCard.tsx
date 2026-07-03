import React, { useState } from 'react';
import { HelpCircle, Send } from 'lucide-react';
import { NeedsInputItem } from './eventReducer';
import { useRunStore } from '../../stores/runStore';

export const NeedsInputCard: React.FC<{ item: NeedsInputItem }> = ({ item }) => {
  const { activeRunId, replyNeedsInput, pendingInput } = useRunStore();
  const [text, setText] = useState('');

  const isPending = pendingInput?.id === item.id;

  const handleSubmit = (content: string) => {
    if (activeRunId && isPending) {
      replyNeedsInput(activeRunId, item.id, content);
    }
  };

  return (
    <div className="border border-mode-ask/40 bg-mode-ask/10 rounded-lg p-4 my-4">
      <div className="flex items-start mb-3 text-mode-ask">
        <HelpCircle className="w-5 h-5 mr-2 shrink-0 mt-0.5" />
        <span className="font-medium text-sm">{item.prompt}</span>
      </div>
      
      {isPending ? (
        <div className="space-y-2 mt-3">
          {item.choices && item.choices.length > 0 && (
            <div className="flex flex-wrap gap-2 mb-2">
              {item.choices.map(choice => (
                <button
                  key={choice}
                  onClick={() => handleSubmit(choice)}
                  className="px-3 py-1.5 bg-surface border border-mode-ask/30 rounded text-sm text-text-900 hover:bg-mode-ask hover:text-white transition-colors"
                >
                  {choice}
                </button>
              ))}
            </div>
          )}
          <div className="flex gap-2">
            <input 
              type="text"
              value={text}
              onChange={(e) => setText(e.target.value)}
              placeholder="Or type your answer..."
              className="flex-1 bg-surface border border-mode-ask/30 rounded px-3 py-1.5 text-sm focus:outline-none focus:border-mode-ask"
              onKeyDown={(e) => {
                if (e.key === 'Enter' && text.trim()) {
                  handleSubmit(text.trim());
                }
              }}
            />
            <button
              onClick={() => text.trim() && handleSubmit(text.trim())}
              disabled={!text.trim()}
              className="px-3 py-1.5 bg-mode-ask text-white rounded disabled:opacity-50 flex items-center justify-center"
            >
              <Send className="w-4 h-4" />
            </button>
          </div>
        </div>
      ) : (
        <div className="mt-2 text-xs text-text-400 italic">
          Resolved
        </div>
      )}
    </div>
  );
};
