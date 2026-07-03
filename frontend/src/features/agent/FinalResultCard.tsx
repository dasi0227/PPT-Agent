import React from 'react';
import { CheckCircle2 } from 'lucide-react';
import { FinalResultItem } from './eventReducer';

export const FinalResultCard: React.FC<{ item: FinalResultItem }> = ({ item }) => {
  return (
    <div className="border-2 border-mode-final/30 bg-mode-final/5 rounded-lg p-4 my-4">
      <div className="flex items-center mb-3">
        <CheckCircle2 className="w-5 h-5 text-mode-final mr-2" />
        <span className="font-semibold text-text-900 text-base">最终交付</span>
      </div>
      <div className="text-sm text-text-600 bg-surface p-3 rounded border border-mode-final/20 shadow-sm">
        <pre className="whitespace-pre-wrap font-sans">
          {JSON.stringify(item.result, null, 2)}
        </pre>
      </div>
    </div>
  );
};
