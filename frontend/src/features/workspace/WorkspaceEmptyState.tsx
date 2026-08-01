import { Layers } from 'lucide-react';

export const WorkspaceEmptyState = () => {
  return (
    <div className="flex-1 flex flex-col items-center justify-center bg-background text-text-900 h-full w-full">
      <Layers className="w-14 h-14 text-accent mb-5 opacity-90" />
      <h1 className="text-xl font-semibold mb-2">开始制作演示文稿</h1>
      <p className="text-text-400 mb-6">点击顶栏「+」新建或打开项目</p>
      
      {/* 按钮会被提取到外部，这里直接展示占位 */}
      <button 
        className="h-8 px-4 bg-accent text-white rounded-md hover:bg-accent/90 font-medium"
        onClick={() => {
          // Trigger ProjectPickerModal, will implement in M5
          document.dispatchEvent(new CustomEvent('open-project-picker'));
        }}
      >
        + 新建 / 打开项目
      </button>
    </div>
  );
};
