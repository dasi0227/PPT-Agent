import React from 'react';
import { AlertTriangle, CheckCircle2 } from 'lucide-react';
import type { StructuredOutcome } from '../../api/types';
import { Disclosure } from '../../components/ui/primitives';
import { FinalResultItem } from './eventReducer';
import { MarkdownMessage } from './MarkdownMessage';
import { artifactLabels, strategyLabels, targetLabel } from './runtimeLabels';

export const FinalResultCard: React.FC<{ item: FinalResultItem }> = ({ item }) => {
  const result = item.result && typeof item.result === 'object' ? item.result as StructuredOutcome : null;
  const plainSummary = typeof item.result === 'string' ? item.result : null;
  const issues = result?.issues ?? [];
  const affected = result?.affected ?? [];

  return (
    <div className="my-4 border-l-2 border-success pl-3 text-sm text-text-900">
      <div className="mb-2 flex items-center">
        <CheckCircle2 className="mr-2 h-5 w-5 shrink-0 text-success" strokeWidth={1.75} />
        <span className="font-semibold">执行结果</span>
        {issues.length > 0 && (
          <span className="ml-auto flex items-center text-xs font-medium text-warning">
            <AlertTriangle className="mr-1 h-3.5 w-3.5" />
            {issues.length} 项需要注意
          </span>
        )}
      </div>
      <div className="space-y-2">
        {plainSummary && <MarkdownMessage content={plainSummary} />}
        {result?.summary && (
          <section>
            <h3 className="mb-1 text-xs font-medium text-text-600">完成内容</h3>
            <MarkdownMessage content={result.summary} />
          </section>
        )}
        {result?.target && (
          <p><span className="text-text-600">影响范围：</span>{targetLabel(result.target.artifact, result.target.level)}</p>
        )}
        {affected.length > 0 && (
          <p>
            <span className="text-text-600">产物：</span>
            {affected.map((artifact) => artifactLabels[String(artifact.kind)] ?? String(artifact.kind ?? artifact.id ?? '产物')).join('、')}
          </p>
        )}
        {result?.strategy && (
          <p><span className="text-text-600">执行策略：</span>{strategyLabels[result.strategy]}</p>
        )}
        <p><span className="text-text-600">验证结果：</span>{issues.length === 0 ? '已通过' : `${issues.length} 项待处理`}</p>
        {issues.length > 0 && (
          <Disclosure label="查看验证问题">
            <ul className="space-y-1 text-xs text-warning">
              {issues.map((issue, index) => (
                <li key={`${issue.code ?? 'issue'}-${index}`}>
                  {issue.code ? `[${issue.code}] ` : ''}{issue.evidence ?? issue.message ?? '验证问题'}
                </li>
              ))}
            </ul>
          </Disclosure>
        )}
        <p className="text-xs text-text-600">你可以继续说明需要调整的内容。</p>
      </div>
    </div>
  );
};
