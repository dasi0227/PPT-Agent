import React from 'react';
import { PanelLeft, PanelRight } from 'lucide-react';
import { useUIStore } from '../../stores/uiStore';

export const PanelToggleButtons: React.FC = () => {
  const { leftPanelHidden, rightPanelHidden, toggleLeftPanel, toggleRightPanel } = useUIStore();

  return (
    <div className="flex items-center gap-1 ml-auto pr-2">
      <button 
        aria-label={leftPanelHidden ? '展开左侧目录' : '隐藏左侧目录'}
        onClick={toggleLeftPanel}
        className="p-1.5 rounded hover:bg-black/5 text-text-600 transition-colors"
      >
        <PanelLeft className="w-4 h-4" />
      </button>
      <button 
        aria-label={rightPanelHidden ? '展开右侧对话' : '隐藏右侧对话'}
        onClick={toggleRightPanel}
        className="p-1.5 rounded hover:bg-black/5 text-text-600 transition-colors"
      >
        <PanelRight className="w-4 h-4" />
      </button>
    </div>
  );
};
