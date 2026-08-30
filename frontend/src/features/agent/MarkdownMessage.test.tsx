import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { MarkdownMessage, normalizeMarkdownSectionSpacing } from './MarkdownMessage';

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

  it('separates numbered bold section titles from following prose', () => {
    const markdown = '**1. 主题与定位**\n正文内容\n\n**2. 页面结构**\n- 第一页';
    render(<MarkdownMessage content={markdown} />);

    const titleParagraph = screen.getByText('1. 主题与定位').closest('p');
    const bodyParagraph = screen.getByText('正文内容').closest('p');
    expect(titleParagraph).not.toBe(bodyParagraph);
    expect(screen.getByText('2. 页面结构').closest('p')).not.toBeNull();
    expect(screen.getByText('第一页').closest('li')).not.toBeNull();
  });

  it('does not normalize heading-like text inside fenced code blocks', () => {
    const markdown = '```md\n**1. 主题与定位**\n正文内容\n```';

    expect(normalizeMarkdownSectionSpacing(markdown)).toBe(markdown);
  });
});
