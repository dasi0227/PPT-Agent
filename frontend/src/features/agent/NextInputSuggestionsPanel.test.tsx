import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { NextInputSuggestionsPanel } from './NextInputSuggestionsPanel';

describe('NextInputSuggestionsPanel', () => {
  it('renders full plain-text buttons and fills only the selected value', () => {
    const select = vi.fn();
    const longSuggestion = '继续完善第 3 页的结论与证据关系，并统一整页的信息层级和文字换行';
    render(<NextInputSuggestionsPanel items={[longSuggestion, '<strong>检查叙事</strong>']} onSelect={select} />);

    expect(screen.getByText(longSuggestion)).toHaveClass('whitespace-normal', 'break-words');
    expect(document.querySelector('strong')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: /2\. <strong>检查叙事<\/strong>/ }));
    expect(select).toHaveBeenCalledWith('<strong>检查叙事</strong>');
    expect(select).toHaveBeenCalledTimes(1);
  });
});
