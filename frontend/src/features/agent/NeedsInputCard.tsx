import React, { useState } from 'react';
import { HelpCircle, Send } from 'lucide-react';
import { NeedsInputItem } from './eventReducer';
import { useRunStore } from '../../stores/runStore';
import { useActiveThreadId, useActiveSession } from './useActiveSession';
import { MarkdownMessage } from './MarkdownMessage';

export const NeedsInputCard: React.FC<{ item: NeedsInputItem }> = ({ item }) => {
  const threadId = useActiveThreadId();
  const { activeRunId, pendingInput } = useActiveSession();
  const replyNeedsInput = useRunStore((s) => s.replyNeedsInput);
  const [text, setText] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [answered, setAnswered] = useState(false);

  const isPending = pendingInput?.id === item.id && !answered;

  const handleSubmit = async (content: string) => {
    if (!threadId || !activeRunId || !isPending || submitting) return;
    setSubmitting(true);
    const accepted = await replyNeedsInput(threadId, activeRunId, item.id, content);
    if (accepted) setAnswered(true);
    setSubmitting(false);
  };

  return (
    <div className="border border-warning/40 bg-warning-soft rounded-lg p-4 my-4">
      <div className="flex items-start mb-3 text-warning">
        <HelpCircle className="w-5 h-5 mr-2 shrink-0 mt-0.5" />
        <div className="font-medium text-sm min-w-0 flex-1">
          <MarkdownMessage content={item.prompt} />
        </div>
      </div>
      
      {isPending ? (
        <div className="space-y-2 mt-3">
          {item.choices && item.choices.length > 0 && (
            <div className="flex flex-wrap gap-2 mb-2">
              {item.choices.map(choice => (
                <button
                  key={choice}
                  onClick={() => void handleSubmit(choice)}
                  disabled={submitting}
                  className="px-3 py-1.5 bg-surface border border-warning/30 rounded text-sm text-text-900 hover:bg-warning hover:text-white transition-colors"
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
              placeholder="输入你的回答"
              className="flex-1 bg-surface border border-warning/30 rounded px-3 py-1.5 text-sm focus:border-warning"
              onKeyDown={(e) => {
                if (e.key === 'Enter' && text.trim()) {
                  void handleSubmit(text.trim());
                }
              }}
            />
            <button
              onClick={() => text.trim() && void handleSubmit(text.trim())}
              disabled={!text.trim() || submitting}
              aria-label="提交回答"
              className="px-3 py-1.5 bg-warning text-white rounded disabled:opacity-50 flex items-center justify-center"
            >
              <Send className="w-4 h-4" />
            </button>
          </div>
        </div>
      ) : (
        <div className="mt-2 text-xs text-text-400 italic">
          已回答
        </div>
      )}
    </div>
  );
};
