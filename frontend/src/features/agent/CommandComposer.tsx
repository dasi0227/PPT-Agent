import React, { useState } from 'react';
import { Send } from 'lucide-react';
import { useRunStore } from '../../stores/runStore';
import { useProjectStore } from '../../stores/projectStore';
import { useDeckStore } from '../../stores/deckStore';
import { projectsApi } from '../../api/projects';

export const CommandComposer: React.FC = () => {
  const [text, setText] = useState('');
  const { activeProjectId, threadsByProjectId } = useProjectStore();
  const { currentPage } = useDeckStore();
  const { createRun, status } = useRunStore();

  const disabled = !activeProjectId || status === 'running' || status === 'needs_input';

  const handleSubmit = async () => {
    if (!text.trim() || disabled) return;

    let mode: 'normal' | 'talk' | 'ask' = 'normal';
    let instruction = text.trim();

    if (instruction.startsWith('/talk ')) {
      mode = 'talk';
      instruction = instruction.replace('/talk ', '');
    } else if (instruction.startsWith('/ask ')) {
      mode = 'ask';
      instruction = instruction.replace('/ask ', '');
    }

    const threads = threadsByProjectId[activeProjectId] || [];
    let threadId = threads[0]?.id;

    if (!threadId) {
      try {
        const newThread = await projectsApi.createThread(activeProjectId);
        threadId = newThread.id;
        useProjectStore.getState().loadProjectThreads(activeProjectId);
      } catch (err) {
        console.error('Failed to create thread', err);
        return;
      }
    }

    await createRun(threadId, {
      kind: 'generate',
      instruction,
      scope: '/current',
      mode,
      page_index: currentPage
    });
    
    setText('');
  };

  return (
    <div className="p-4 border-t border-border bg-background">
      <div className="relative bg-surface rounded-lg border border-border shadow-sm focus-within:border-mode-normal focus-within:ring-1 focus-within:ring-mode-normal transition-all">
        <textarea
          value={text}
          onChange={(e) => setText(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
              e.preventDefault();
              handleSubmit();
            }
          }}
          placeholder={status === 'needs_input' ? "Waiting for your input above..." : "Ask me to edit this slide..."}
          disabled={disabled}
          className="w-full bg-transparent resize-none p-3 max-h-32 min-h-[60px] text-sm text-text-900 placeholder:text-text-400 focus:outline-none disabled:opacity-50"
          rows={2}
        />
        <div className="flex items-center justify-between px-3 pb-2">
          <div className="flex gap-2">
            <span className="text-[10px] text-text-400 bg-background px-1.5 py-0.5 rounded uppercase">/current</span>
            <span className="text-[10px] text-text-400 bg-background px-1.5 py-0.5 rounded uppercase">Page {currentPage + 1}</span>
          </div>
          <button
            onClick={handleSubmit}
            disabled={!text.trim() || disabled}
            className="p-1.5 bg-mode-normal text-white rounded-md disabled:opacity-50 disabled:bg-text-400 hover:opacity-90 transition-opacity"
          >
            <Send className="w-4 h-4" />
          </button>
        </div>
      </div>
      <div className="mt-2 text-center text-xs text-text-400 flex items-center justify-center">
        Use <kbd className="mx-1 px-1 rounded bg-black/5 font-sans border border-border">/talk</kbd> or <kbd className="mx-1 px-1 rounded bg-black/5 font-sans border border-border">/ask</kbd> to change modes
      </div>
    </div>
  );
};
