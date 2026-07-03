import React from 'react';
import { FileDown, Code, Image as ImageIcon } from 'lucide-react';
import { ArtifactItem } from './eventReducer';

export const ArtifactCard: React.FC<{ item: ArtifactItem }> = ({ item }) => {
  return (
    <div className="flex items-center p-2 rounded border border-border bg-surface text-sm">
      <div className="w-8 h-8 rounded bg-background flex items-center justify-center mr-3 text-text-600">
        {item.artifact_type === 'slide_html' ? <Code className="w-4 h-4" /> : 
         item.artifact_type === 'design' ? <ImageIcon className="w-4 h-4" /> : 
         <FileDown className="w-4 h-4" />}
      </div>
      <div className="flex-1 min-w-0">
        <div className="font-medium text-text-900 truncate">
          {item.artifact_type}
        </div>
        <div className="text-xs text-text-400 truncate">
          {item.ref} {item.page_index !== undefined ? `(Page ${item.page_index + 1})` : ''}
        </div>
      </div>
      <div className="px-2 py-0.5 rounded-full bg-background text-[10px] text-text-400 border border-border uppercase">
        中间产物
      </div>
    </div>
  );
};
