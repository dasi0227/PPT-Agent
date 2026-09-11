import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, test, vi } from 'vitest';
import { ExportButton } from './ExportButton';

describe('ExportButton', () => {
  test('offers exactly the three whole-deck export formats', async () => {
    const user = userEvent.setup();
    const onExport = vi.fn();
    render(<ExportButton disabled={false} onExport={onExport} />);
    await user.click(screen.getByRole('button', { name: '导出' }));
    expect(screen.getAllByRole('menuitem').map((item) => item.textContent)).toEqual(['导出 PNG', '导出 PDF', '导出 HTML']);
    await user.click(screen.getByRole('menuitem', { name: '导出 PDF' }));
    expect(onExport).toHaveBeenCalledWith('pdf');
  });
});
