import React from 'react';
import { Sparkles } from 'lucide-react';
import { useComposerStore } from '../../stores/composerStore';

export const EmptyState: React.FC = () => {
  const requestOutlineFocus = useComposerStore((s) => s.requestOutlineFocus);

  return (
    <div className="flex flex-col items-center justify-center text-center p-8 max-w-sm">
      <div className="w-12 h-12 rounded-full bg-mode-normal/10 flex items-center justify-center mb-4">
        <Sparkles className="w-6 h-6 text-mode-normal" />
      </div>
      <h3 className="text-base font-medium text-text-900 mb-1">还没有内容</h3>
      <p className="text-sm text-text-600 mb-5">
        先让 Agent 生成一份大纲，再逐页构建。
      </p>
      <button
        type="button"
        onClick={requestOutlineFocus}
        className="px-4 py-2 rounded-md bg-mode-normal text-white text-sm font-medium hover:opacity-90 transition-opacity"
      >
        描述主题并生成大纲
      </button>
    </div>
  );
};
