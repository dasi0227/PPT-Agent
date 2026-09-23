import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { RepositoryPreviewCache } from './RepositoryPreviewCache';

describe('repository preview lifetime', () => {
  it('mounts on first visit and preserves document identity and local state on return', () => {
    const entries = ['cover', 'content', 'chart'].map(key => ({
      key, content: <div><iframe title={key} /><input aria-label={`${key} state`} defaultValue="initial" /></div>,
    }));
    const { rerender } = render(<RepositoryPreviewCache entries={entries} activeKey="cover" />);
    const original = screen.getByTitle('cover');
    const input = screen.getByRole('textbox') as HTMLInputElement;
    input.value = 'retained';
    expect(screen.queryByTitle('chart')).toBeNull();

    rerender(<RepositoryPreviewCache entries={entries} activeKey="content" />);
    expect(screen.getByTitle('cover')).toBe(original);
    expect(original.closest('[data-preview-active]')).toHaveAttribute('inert');
    expect(screen.getAllByRole('textbox')).toHaveLength(1);
    rerender(<RepositoryPreviewCache entries={entries} activeKey="cover" />);
    expect(screen.getByTitle('cover')).toBe(original);
    expect(screen.getByRole('textbox')).toHaveValue('retained');
    expect(original.closest('[data-preview-active]')).not.toHaveAttribute('inert');
  });

  it('evicts replaced or removed resources and bounds retained documents', () => {
    const entries = Array.from({ length: 10 }, (_, index) => ({ key: String(index), content: <iframe title={`preview-${index}`} /> }));
    const { rerender, container } = render(<RepositoryPreviewCache entries={entries} activeKey="0" />);
    for (let index = 1; index < entries.length; index++) {
      rerender(<RepositoryPreviewCache entries={entries} activeKey={String(index)} />);
    }
    expect(container.querySelectorAll('iframe')).toHaveLength(9);
    expect(screen.queryByTitle('preview-0')).toBeNull();
    const previous = screen.getByTitle('preview-9');
    rerender(<RepositoryPreviewCache entries={[{ key: '9:updated', content: <iframe title="preview-9" /> }]} activeKey="9:updated" />);
    expect(container.querySelectorAll('iframe')).toHaveLength(1);
    expect(screen.getByTitle('preview-9')).not.toBe(previous);
  });
});
