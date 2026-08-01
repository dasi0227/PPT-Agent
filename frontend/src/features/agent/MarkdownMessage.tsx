import React from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';

interface MarkdownMessageProps {
  content: string;
}

export const MarkdownMessage: React.FC<MarkdownMessageProps> = ({ content }) => {
  return (
    <div className="prose prose-sm max-w-none text-text-900 prose-p:leading-relaxed prose-pre:bg-surface prose-pre:border prose-pre:border-border prose-pre:text-text-900 prose-a:text-accent">
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
