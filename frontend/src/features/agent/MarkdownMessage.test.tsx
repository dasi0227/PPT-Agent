import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { MarkdownMessage } from './MarkdownMessage';

describe('MarkdownMessage', () => {
  it('renders tables', () => {
    const markdown = `
| Header 1 | Header 2 |
| -------- | -------- |
| Cell 1   | Cell 2   |
    `;
    render(<MarkdownMessage content={markdown} />);
    expect(screen.getByText('Header 1')).toBeInTheDocument();
    expect(screen.getByText('Cell 1')).toBeInTheDocument();
  });

  it('renders links and code blocks', () => {
    const markdown = `[Link](https://example.com)\n\n\`\`\`js\nconst x = 1;\n\`\`\``;
    render(<MarkdownMessage content={markdown} />);
    expect(screen.getByText('Link')).toHaveAttribute('href', 'https://example.com');
    // Using simple match for code content
    expect(document.querySelector('code')).toBeInTheDocument();
  });

  it('renders images', () => {
    const markdown = `![Alt Text](https://example.com/img.png)`;
    render(<MarkdownMessage content={markdown} />);
    const img = screen.getByAltText('Alt Text');
    expect(img).toHaveAttribute('src', 'https://example.com/img.png');
  });

  it('keeps markdown dividers compact in the agent panel', () => {
    const { container } = render(<MarkdownMessage content={'上文\n\n---\n\n下文'} />);
    expect(container.querySelector('hr')).toBeInTheDocument();
    expect(container.querySelector('.prose')).toHaveClass('prose-hr:my-4', 'prose-hr:border-border');
  });

  it('uses compact indentation for nested lists', () => {
    const markdown = '1. 演示设定\n   - 标题\n   - 目标';
    const { container } = render(<MarkdownMessage content={markdown} />);
    const prose = container.querySelector('.prose');

    expect(prose).toHaveClass(
      'prose-ul:pl-3',
      'prose-ol:pl-4',
      '[&_li>ul]:pl-2',
      '[&_li>ol]:pl-2',
    );
  });
});
