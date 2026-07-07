import React from 'react';
import { CheckCircle2, AlertTriangle } from 'lucide-react';
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

const Field: React.FC<{ label: string; value: React.ReactNode }> = ({ label, value }) => (
  <div className="flex items-baseline gap-2">
    <span className="text-xs text-text-400 shrink-0 w-16">{label}</span>
    <span className="text-sm text-text-900 min-w-0 break-words">{value}</span>
  </div>
);

export const FinalResultCard: React.FC<{ item: FinalResultItem }> = ({ item }) => {
  const result = item.result;
  const structured = isStructuredResult(result);
  const warnings: Warning[] = structured && Array.isArray(result.warnings) ? result.warnings : [];
  const summary: string | undefined = !structured
    ? (typeof result === 'string' ? result : result?.summary)
    : undefined;

  return (
    <div className="border-2 border-mode-final/30 bg-mode-final/5 rounded-lg p-4 my-4 shadow-sm">
      <div className="flex items-center mb-3">
        <CheckCircle2 className="w-5 h-5 text-mode-final mr-2 shrink-0" />
        <span className="font-semibold text-text-900 text-base">最终交付</span>
        {structured && warnings.length > 0 && (
          <span className="ml-auto flex items-center text-xs font-medium text-mode-ask">
            <AlertTriangle className="w-3.5 h-3.5 mr-1" />
            {warnings.length} 项告警
          </span>
        )}
      </div>

      {structured ? (
        <div className="space-y-2 bg-surface p-3 rounded border border-mode-final/20">
          <Field label="页数" value={<span className="tabular-nums">{result.slide_count}</span>} />
          {result.theme && <Field label="主题" value={result.theme} />}
          {result.signature && <Field label="签名" value={result.signature} />}
          {result.design_spec_ref && <Field label="设计规约" value={<code className="text-xs text-text-600">{result.design_spec_ref}</code>} />}
        </div>
      ) : (
        <div className="text-sm text-text-600 bg-surface p-3 rounded border border-mode-final/20 break-words">
          {summary && summary.trim().length > 0
            ? <MarkdownMessage content={summary} />
            : <span className="text-text-400">已完成</span>}
        </div>
      )}

      {structured && warnings.length > 0 && (
        <div className="mt-3 border-l-2 border-mode-ask pl-3">
          <div className="text-xs font-medium text-mode-ask mb-1.5">未完全达成的页面</div>
          <ul className="space-y-1">
            {warnings.map((w, i) => (
              <li key={i} className="text-sm text-text-600">
                <span className="font-medium text-text-900">第 {w.page_index + 1} 页</span>
                <span className="text-xs text-text-400 ml-1.5">[{w.code}]</span>
                <span className="ml-1.5 break-words">{w.message}</span>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
};
