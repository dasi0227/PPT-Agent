import React, { useState } from 'react';
import { ChevronDown, ChevronRight, Wrench, CheckCircle2, XCircle, Loader2 } from 'lucide-react';
import { ToolCallItem } from './eventReducer';
import { cn } from '../../lib/utils';
import { ArtifactCard } from './ArtifactCard';

export const ToolCallCard: React.FC<{ item: ToolCallItem }> = ({ item }) => {
  const [expanded, setExpanded] = useState(false);

  const getStatusIcon = () => {
    if (item.status === 'running') return <Loader2 className="w-4 h-4 animate-spin text-text-400" />;
    if (item.status === 'success') return <CheckCircle2 className="w-4 h-4 text-mode-normal" />;
    return <XCircle className="w-4 h-4 text-mode-error" />;
  };

  return (
    <div className="border border-border rounded-md bg-surface my-2 shadow-sm relative overflow-hidden">
      <div className={cn(
        "absolute left-0 top-0 bottom-0 w-1",
        item.status === 'running' ? "bg-text-400" : item.status === 'success' ? "bg-mode-normal" : "bg-mode-error"
      )} />
      <div className="pl-3">
        <button 
          onClick={() => setExpanded(!expanded)}
          className="w-full flex items-center px-3 py-2 text-sm text-text-900 hover:bg-black/5 transition-colors"
        >
          {expanded ? <ChevronDown className="w-4 h-4 mr-1 text-text-400" /> : <ChevronRight className="w-4 h-4 mr-1 text-text-400" />}
          <Wrench className="w-4 h-4 mr-2 text-text-600" />
          <span className="font-medium mr-2">{item.tool}</span>
          <span className="text-text-400 truncate flex-1 text-left text-xs">
            {JSON.stringify(item.args).substring(0, 40)}...
          </span>
          <div className="ml-2">
            {getStatusIcon()}
          </div>
        </button>
        {expanded && (
          <div className="px-4 py-3 border-t border-border text-xs bg-background/50 overflow-x-auto">
            <div className="font-semibold text-text-600 mb-1">Arguments:</div>
            <pre className="text-text-900 mb-3">{JSON.stringify(item.args, null, 2)}</pre>
            
            {item.observation && (
              <>
                <div className="font-semibold text-text-600 mb-1">Observation:</div>
                <pre className="text-text-900">{JSON.stringify(item.observation, null, 2)}</pre>
              </>
            )}
          </div>
        )}
      </div>
      {item.artifacts && item.artifacts.length > 0 && (
        <div className="border-t border-border bg-background p-2 pl-4 flex flex-col gap-2">
          {item.artifacts.map(art => (
            <ArtifactCard key={art.id} item={art} />
          ))}
        </div>
      )}
    </div>
  );
};
