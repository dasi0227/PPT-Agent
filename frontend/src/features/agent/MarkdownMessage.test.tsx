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

  it('keeps list markers visibly indented from the message edge', () => {
    const markdown = '1. 演示设定\n   - 标题\n   - 目标';
    const { container } = render(<MarkdownMessage content={markdown} />);
    const prose = container.querySelector('.prose');

    expect(prose).toHaveClass(
      'prose-ul:pl-6',
      'prose-ol:pl-6',
      '[&_li>ul]:pl-5',
      '[&_li>ol]:pl-5',
    );
  });

  it('keeps standalone bold text as emphasis and preserves explicit headings', () => {
    const emphasizedText = '是什么 → 什么不该做 → 六步做法 → 怎么判断做好了 → 常见坑 → 收尾';
    const markdown = `### 叙事主线\n\n**${emphasizedText}**\n\n### 页面结构\n\n**2. 页面结构**\n- 第一页`;
    render(<MarkdownMessage content={markdown} />);

    expect(screen.getByText(emphasizedText).tagName).toBe('STRONG');
    expect(screen.getByText('2. 页面结构').tagName).toBe('STRONG');
    expect(screen.queryByRole('heading', { name: emphasizedText })).not.toBeInTheDocument();
    expect(screen.getByRole('heading', { level: 3, name: '叙事主线' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { level: 3, name: '页面结构' })).toBeInTheDocument();
    expect(screen.getByText('第一页').closest('li')).not.toBeNull();
  });

  it('does not normalize heading-like text inside fenced code blocks', () => {
    const markdown = '```md\n**1. 主题与定位**\n正文内容\n```';

    const { container } = render(<MarkdownMessage content={markdown} />);

    expect(container.querySelector('code')).toHaveTextContent('**1. 主题与定位**');
    expect(screen.queryByRole('heading')).not.toBeInTheDocument();
  });
});
