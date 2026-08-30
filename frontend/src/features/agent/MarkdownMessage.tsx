import React from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { cn } from '../../lib/utils';

interface MarkdownMessageProps {
  content: string;
  className?: string;
}

const numberedStrongHeading = /^\s*\*\*(?:\d+[.)、．]?|[一二三四五六七八九十]+[、.．])\s*.+\*\*\s*$/;

export function normalizeMarkdownSectionSpacing(content: string): string {
  const lines = content.split('\n');
  const normalized: string[] = [];
  let fenceMarker: '`' | '~' | null = null;

  lines.forEach((line, index) => {
    const fence = line.match(/^\s*(`{3,}|~{3,})/);
    if (fence) {
      const marker = fence[1][0] as '`' | '~';
      fenceMarker = fenceMarker === marker ? null : fenceMarker ?? marker;
      normalized.push(line);
      return;
    }

    normalized.push(line);
    const nextLine = lines[index + 1];
    if (
      !fenceMarker
      && numberedStrongHeading.test(line)
      && nextLine !== undefined
      && nextLine.trim() !== ''
    ) {
      normalized.push('');
    }
  });

  return normalized.join('\n');
}

export const MarkdownMessage: React.FC<MarkdownMessageProps> = ({ content, className }) => {
  return (
    <div className={cn(
      'prose prose-sm max-w-none text-text-900 prose-headings:mb-2 prose-headings:mt-4 prose-headings:font-semibold prose-p:my-2 prose-p:leading-relaxed prose-ul:my-2 prose-ul:pl-3 prose-ol:my-2 prose-ol:pl-4 prose-li:my-0.5 [&_li>ul]:pl-2 [&_li>ol]:pl-2 prose-hr:my-4 prose-hr:border-border prose-strong:text-text-900 prose-table:my-3 prose-th:border prose-th:border-border prose-th:bg-panel-muted prose-th:px-2 prose-th:py-1 prose-td:border prose-td:border-border prose-td:px-2 prose-td:py-1 prose-pre:border prose-pre:border-border prose-pre:bg-surface prose-pre:text-text-900 prose-code:text-text-900 prose-a:text-accent',
      className,
    )}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          a: ({ node, ...props }) => (
            <a {...props} target="_blank" rel="noopener noreferrer" />
          ),
          img: ({ node, ...props }) => (
            <img {...props} loading="lazy" className="max-w-full h-auto rounded" />
          )
        }}
      >
        {normalizeMarkdownSectionSpacing(content)}
      </ReactMarkdown>
    </div>
  );
};
