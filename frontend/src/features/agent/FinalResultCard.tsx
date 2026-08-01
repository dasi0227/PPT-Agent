import React from 'react';
import { AlertTriangle, CheckCircle2 } from 'lucide-react';
import { FinalResultItem } from './eventReducer';
import { MarkdownMessage } from './MarkdownMessage';

function isStructuredResult(r: any): boolean {
  return r != null && typeof r === 'object' && typeof r.status === 'string' && r.target != null;
}

export const FinalResultCard: React.FC<{ item: FinalResultItem }> = ({ item }) => {
  const result = item.result;
  const structured = isStructuredResult(result);
  const issues: any[] = structured && Array.isArray(result.issues) ? result.issues : [];
  const summary: string | undefined = !structured
    ? (typeof result === 'string' ? result : result?.summary)
    : undefined;

  return (
    <div className="flex justify-start my-4">
      <div className="max-w-[85%] bg-surface border border-border rounded-lg p-3 shadow-sm text-sm text-text-900">
        <div className="mb-3 flex items-center">
          <CheckCircle2 className="mr-2 h-5 w-5 shrink-0 text-mode-final" />
          <span className="font-semibold">最终交付</span>
          {structured && issues.length > 0 && (
            <span className="ml-auto flex items-center text-xs font-medium text-mode-ask">
              <AlertTriangle className="mr-1 h-3.5 w-3.5" />
              {issues.length} 项问题
            </span>
          )}
        </div>
        {structured ? (
          <div className="space-y-2">
            {result.target && <p><strong>目标：</strong> {result.target.artifact} / {result.target.level}</p>}
            {result.strategy && <p><strong>策略：</strong> {result.strategy}</p>}
            {result.operation && <p><strong>操作：</strong> {result.operation}</p>}
            {Array.isArray(result.affected) && result.affected.length > 0 && (
              <p><strong>影响产物：</strong> {result.affected.map((item: any) => `${item.kind}:${item.id}`).join(', ')}</p>
            )}
            {typeof result.repair_rounds === 'number' && result.repair_rounds > 0 && (
              <p><strong>修复轮次：</strong> {result.repair_rounds}</p>
            )}
            {result.summary && <MarkdownMessage content={String(result.summary)} />}
            {issues.length > 0 && (
              <div className="mt-2 text-mode-ask">
                <div className="flex items-center font-medium mb-1">
                  <AlertTriangle className="w-4 h-4 mr-1" />
                  验证问题
                </div>
                <ul className="list-disc list-inside space-y-1 ml-1 text-xs">
                  {issues.map((issue, i) => (
                    <li key={i}>
                      <span>[{issue.code}]</span>{' '}
                      <span>{issue.evidence || issue.message}</span>
                    </li>
                  ))}
                </ul>
              </div>
            )}
          </div>
        ) : (
          <div className="break-words">
            {summary && summary.trim().length > 0
              ? <MarkdownMessage content={summary} />
              : <span className="text-text-400">已完成</span>}
          </div>
        )}
      </div>
    </div>
  );
};
