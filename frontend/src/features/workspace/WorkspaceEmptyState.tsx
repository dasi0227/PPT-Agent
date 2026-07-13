import { Layers } from 'lucide-react';

export const WorkspaceEmptyState = () => {
  return (
    <div className="flex-1 flex flex-col items-center justify-center bg-background text-text-900 h-full w-full">
      <Layers className="w-16 h-16 text-mode-normal mb-6 opacity-80" />
      <h1 className="text-2xl font-bold mb-2">Welcome back</h1>
      <p className="text-text-400 mb-6">点击顶栏「+」新建或打开项目</p>
      
      {/* 按钮会被提取到外部，这里直接展示占位 */}
      <button 
        className="px-4 py-2 bg-mode-normal text-white rounded-md hover:bg-mode-normal/90 font-medium transition-colors"
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
