import React from 'react';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { MarkdownMessage } from './MarkdownMessage';
import { FinalMessage } from './FinalMessage';
import type { PublicTarget } from '../../api/types';

const markdown = vi.hoisted(() => ({ renders: 0 }));
vi.mock('react-markdown', async (load) => {
  const actual = await load<typeof import('react-markdown')>();
  return { ...actual, default: (props: React.ComponentProps<typeof actual.default>) => {
    markdown.renders += 1;
    return <actual.default {...props} />;
  } };
});

afterEach(() => { cleanup(); markdown.renders = 0; });

describe('timeline rendering work', () => {
  it('keeps unchanged message parsing independent of progress updates', () => {
    const content = '**已经生成的消息**\n\n' + '正文保持不变。'.repeat(100);
    function Messages({ progress }: { progress: number }) {
      return <><span>{progress}</span>{Array.from({ length: 24 }, (_, i) => <MarkdownMessage key={i} content={content} />)}</>;
    }
    const view = render(<Messages progress={0} />);
    const initial = markdown.renders;
    for (let progress = 1; progress <= 26; progress++) view.rerender(<Messages progress={progress} />);
    console.info(`markdown_parse_counts initial=${initial} after_26_updates=${markdown.renders}`);
    expect(markdown.renders).toBe(initial);
    view.unmount();
    render(<MarkdownMessage content="新正文" />);
    expect(screen.getByText('新正文')).toBeTruthy();
  });

  it('mounts file differences only while their disclosure is open', () => {
    const target: PublicTarget = {
      type: 'file', part: 'content', display_name: 'slide.html', insertions: 1,
      diff: { kind: 'text', status: 'modified', filename: 'slide.html', hunks: [{
        old_start: 1, old_count: 0, new_start: 1, new_count: 1,
        rows: [{ kind: 'added', text: 'unique-heavy-diff-line', new_line: 1 }],
      }] },
    };
    render(<FinalMessage item={{ id: 'final1', type: 'final', messageId: 'message1', text: '', timestamp: 1, affectedTargets: [target] }} />);
    const text = () => screen.queryByText('unique-heavy-diff-line');
    console.info(`collapsed_diff_mounted=${text() !== null}`);
    expect(text()).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: /已更改 1 项内容/ }));
    expect(text()).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'slide.html变更' }));
    expect(text()).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'slide.html变更' }));
    expect(text()).toBeNull();
  });
});
