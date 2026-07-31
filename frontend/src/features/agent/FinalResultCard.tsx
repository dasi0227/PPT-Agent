import React from 'react';
import { AlertTriangle, CheckCircle2 } from 'lucide-react';
import { FinalResultItem } from './eventReducer';
import { MarkdownMessage } from './MarkdownMessage';

interface Warning {
  page_index: number;
  code: string;
  message: string;
}

// isStructuredResult 判断 done.result 是否为整套生成的结构化交付（V2 §8.2），
// 否则回退为 {summary} 文本（edit/outline/command 兼容，V2-CONTRACTS §2.4）。
function isStructuredResult(r: any): boolean {
  return r != null && typeof r === 'object' && typeof r.slide_count === 'number';
}

export const FinalResultCard: React.FC<{ item: FinalResultItem }> = ({ item }) => {
  const result = item.result;
  const structured = isStructuredResult(result);
  const warnings: Warning[] = structured && Array.isArray(result.warnings) ? result.warnings : [];
  const summary: string | undefined = !structured
    ? (typeof result === 'string' ? result : result?.summary)
    : undefined;

  return (
    <div className="flex justify-start my-4">
      <div className="max-w-[85%] bg-surface border border-border rounded-lg p-3 shadow-sm text-sm text-text-900">
        <div className="mb-3 flex items-center">
          <CheckCircle2 className="mr-2 h-5 w-5 shrink-0 text-mode-final" />
          <span className="font-semibold">最终交付</span>
          {structured && warnings.length > 0 && (
            <span className="ml-auto flex items-center text-xs font-medium text-mode-ask">
              <AlertTriangle className="mr-1 h-3.5 w-3.5" />
              {warnings.length} 项告警
            </span>
          )}
        </div>
        {structured ? (
          <div className="space-y-2">
            <p><strong>页数：</strong> {result.slide_count}</p>
            {result.theme && <p><strong>主题：</strong> {result.theme}</p>}
            {result.signature && <p><strong>签名：</strong> {result.signature}</p>}
            {result.design_spec_ref && (
              <p><strong>设计规约：</strong> <code className="text-xs text-text-600 bg-black/5 px-1 py-0.5 rounded">{result.design_spec_ref}</code></p>
            )}

            {warnings.length > 0 && (
              <div className="mt-2 text-mode-ask">
                <div className="flex items-center font-medium mb-1">
                  <AlertTriangle className="w-4 h-4 mr-1" />
                  未完全达成的页面
                </div>
                <ul className="list-disc list-inside space-y-1 ml-1 text-xs">
                  {warnings.map((w, i) => (
                    <li key={i}>
                      <span>第 {w.page_index + 1} 页</span>{' '}
                      <span>[{w.code}]</span>{' '}
                      <span>{w.message}</span>
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
