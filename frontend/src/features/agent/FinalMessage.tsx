import React from 'react';
import { CheckCircle2 } from 'lucide-react';
import type { FinalMessageItem } from './eventReducer';
import { MarkdownMessage } from './MarkdownMessage';

function affectedText(item: FinalMessageItem): string {
  if (item.affectedTargets.length === 0) return '';
  const hasOutline = item.affectedTargets.some((target) => target.type === 'deck' && target.part === 'outline');
  const hasDesign = item.affectedTargets.some((target) => target.type === 'deck' && target.part === 'design');
  const slides = new Set(
    item.affectedTargets
      .filter((target) => target.type === 'slide')
      .map((target) => target.slide_id),
  ).size;
  return [
    hasOutline ? '整份结构' : '',
    hasDesign ? '全局设计' : '',
    slides > 0 ? `${slides} 张页面` : '',
  ].filter(Boolean).join('和');
}

export const FinalMessage: React.FC<{ item: FinalMessageItem }> = ({ item }) => {
  const affected = affectedText(item);
  return (
    <article className="pb-4 pt-2 text-sm leading-[1.65] text-text-900">
      <MarkdownMessage content={item.text} />
      {affected && (
        <p className="mt-2 flex items-center gap-1.5 text-xs text-success">
          <CheckCircle2 className="h-4 w-4" strokeWidth={1.75} />
          已更新{affected}
        </p>
      )}
    </article>
  );
};
