import React from 'react';
import { Lightbulb } from 'lucide-react';

export const NewProjectHint: React.FC = () => {
  return (
    <div className="flex-1 flex flex-col items-center justify-center bg-background text-text-900 h-full w-full">
      <div className="w-12 h-12 rounded-full bg-mode-normal/10 flex items-center justify-center mb-4">
        <Lightbulb className="w-6 h-6 text-mode-normal" />
      </div>
      <h2 className="text-xl font-medium mb-2">新建演示文稿</h2>
      <p className="text-sm text-text-600">在右侧输入你想做的 PPT 主题，⌘/Ctrl+Enter 发送即可开始</p>
    </div>
  );
};
