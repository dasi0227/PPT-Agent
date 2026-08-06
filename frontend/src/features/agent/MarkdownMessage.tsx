import React from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';

interface MarkdownMessageProps {
  content: string;
}

export const MarkdownMessage: React.FC<MarkdownMessageProps> = ({ content }) => {
  return (
    <div className="prose prose-sm max-w-none text-text-900 prose-headings:mb-2 prose-headings:mt-4 prose-headings:font-semibold prose-p:my-2 prose-p:leading-relaxed prose-ul:my-2 prose-ol:my-2 prose-li:my-0.5 prose-strong:text-text-900 prose-table:my-3 prose-th:border prose-th:border-border prose-th:bg-panel-muted prose-th:px-2 prose-th:py-1 prose-td:border prose-td:border-border prose-td:px-2 prose-td:py-1 prose-pre:border prose-pre:border-border prose-pre:bg-surface prose-pre:text-text-900 prose-code:text-text-900 prose-a:text-accent">
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
        {content}
      </ReactMarkdown>
    </div>
  );
};
