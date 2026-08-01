import React from 'react';
import { Sparkles } from 'lucide-react';

export const EmptyState: React.FC = () => {
  return (
    <div className="flex flex-col items-center justify-center text-center p-8 max-w-sm">
      <div className="w-12 h-12 rounded-lg bg-accent-soft flex items-center justify-center mb-4">
        <Sparkles className="w-6 h-6 text-accent" />
      </div>
      <h3 className="text-base font-medium text-text-900">暂无内容</h3>
    </div>
  );
};
