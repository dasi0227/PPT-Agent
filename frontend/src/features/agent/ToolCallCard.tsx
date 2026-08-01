import React from 'react';
import { Wrench, CheckCircle2, XCircle, Loader2 } from 'lucide-react';
import { ToolCallItem } from './eventReducer';
import { cn } from '../../lib/utils';
import { ArtifactCard } from './ArtifactCard';
import { Disclosure } from '../../components/ui/primitives';
import { toolLabel } from './runtimeLabels';

export const ToolCallCard: React.FC<{ item: ToolCallItem }> = ({ item }) => {
  const getStatusIcon = () => {
    if (item.status === 'running') return <Loader2 className="w-4 h-4 animate-spin text-text-400" />;
    if (item.status === 'success') return <CheckCircle2 className="w-4 h-4 text-success" />;
    return <XCircle className="w-4 h-4 text-danger" />;
  };

  return (
    <div className="border-l-2 border-border bg-panel my-2 relative">
      <div className={cn(
        "absolute left-0 top-0 bottom-0 w-1",
        item.status === 'running' ? "bg-accent" : item.status === 'success' ? "bg-success" : "bg-danger"
      )} />
      <div className="pl-2">
        <div className="w-full flex items-center px-2 py-2 text-sm text-text-900">
          <Wrench className="w-4 h-4 mr-2 text-text-600" />
          <span className="font-medium mr-2">{toolLabel(item.tool)}</span>
          <span className="text-text-400 truncate flex-1 text-left text-xs">{item.observation ? String(item.observation) : '执行中'}</span>
          <div className="ml-2">
            {getStatusIcon()}
          </div>
        </div>
        <div className="px-2 pb-2">
          <Disclosure label="技术详情">
            <div className="font-semibold text-text-600 mb-1">工具：{item.tool}</div>
            <pre className="overflow-x-auto text-text-900 mb-3">{JSON.stringify(item.args, null, 2)}</pre>
            {item.observation != null && (
              <>
                <div className="font-semibold text-text-600 mb-1">执行结果</div>
                <pre className="overflow-x-auto text-text-900">{JSON.stringify(item.observation, null, 2)}</pre>
              </>
            )}
          </Disclosure>
        </div>
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
