import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { PickerModal } from './modal-picker';

describe('PickerModal', () => {
  const items = [
    { id: '1', name: 'Item 1' },
    { id: '2', name: 'Item 2' },
  ];

  it('renders items and handles search and keyboard navigation', () => {
    const onPick = vi.fn();
    render(
      <PickerModal
        open={true}
        onOpenChange={() => {}}
        title="Pick an Item"
        items={items}
        keyOf={i => i.id}
        searchOf={i => i.name}
        renderItem={i => <span>{i.name}</span>}
        onPick={onPick}
      />
    );

    expect(screen.getByText('Pick an Item')).toBeInTheDocument();
    expect(screen.getByText('Item 1')).toBeInTheDocument();
    expect(screen.getByText('Item 2')).toBeInTheDocument();

    const input = screen.getByPlaceholderText('搜索项目');
    fireEvent.change(input, { target: { value: 'Item 2' } });

    expect(screen.queryByText('Item 1')).not.toBeInTheDocument();
    expect(screen.getByText('Item 2')).toBeInTheDocument();

    // Arrow down (should stay on index 0 since length is 1)
    fireEvent.keyDown(input, { key: 'ArrowDown' });
    // Enter
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(onPick).toHaveBeenCalledWith(items[1]);
  });

  it('waits for confirmation when a confirmation label is provided', () => {
    const onPick = vi.fn();
    render(
      <PickerModal
        open
        onOpenChange={() => {}}
        title="Pick an Item"
        confirmationLabel="Open Item"
        items={items}
        keyOf={i => i.id}
        searchOf={i => i.name}
        renderItem={i => <span>{i.name}</span>}
        onPick={onPick}
      />,
    );

    const confirm = screen.getByRole('button', { name: 'Open Item' });
    expect(confirm).toBeDisabled();

    fireEvent.click(screen.getByRole('button', { name: 'Item 2' }));
    expect(onPick).not.toHaveBeenCalled();
    expect(confirm).toBeEnabled();

    fireEvent.click(confirm);
    expect(onPick).toHaveBeenCalledWith(items[1]);
  });
});
