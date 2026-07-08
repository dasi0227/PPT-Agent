import React from 'react';

// ThinkingBubble：agent 尚未回复任何内容时紧跟最后一条 user_turn 展示的左对齐气泡，
// 使 running 状态在 Timeline 主区就可见，而不是仅靠头部 12px spinner。
// 三点脉动动画通过错峰 animate-bounce 实现（Tailwind 内建关键帧）。
export const ThinkingBubble: React.FC = () => {
  return (
    <div className="flex justify-start" aria-live="polite">
      <div className="max-w-[85%] rounded-lg bg-mode-normal/10 border border-mode-normal/20 px-3 py-2 flex items-center gap-2 text-sm text-mode-normal">
        <span className="flex items-center gap-0.5" aria-hidden>
          <span className="w-1.5 h-1.5 rounded-full bg-mode-normal animate-bounce [animation-delay:-0.2s]" />
          <span className="w-1.5 h-1.5 rounded-full bg-mode-normal animate-bounce [animation-delay:-0.1s]" />
          <span className="w-1.5 h-1.5 rounded-full bg-mode-normal animate-bounce" />
        </span>
        <span>正在思考...</span>
      </div>
    </div>
  );
};
