import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import { FileOpenButton } from './FileOpenButton';
import { useFileSettingsStore } from '../../stores/fileSettingsStore';
const { fetchClient } = vi.hoisted(() => ({ fetchClient: vi.fn().mockResolvedValue(undefined) }));
vi.mock('../../api/client', () => ({ fetchClient }));
it('shows the current action and posts the file endpoint without navigating', async () => {
  useFileSettingsStore.setState({ value: { open_with: 'finder', custom_app_path: '', custom_apps: [], revision: 2, supported: true } });
  render(<FileOpenButton url="/api/v1/files/open?path=%2Ftmp%2Fslide.html" label="幻灯片">打开</FileOpenButton>);
  const button = screen.getByRole('button', { name: '幻灯片：在访达中显示' });
  expect(button).toHaveAttribute('title', '在访达中显示');
  expect(screen.queryByRole('link')).not.toBeInTheDocument();
  fireEvent.click(button);
  await waitFor(() => expect(fetchClient).toHaveBeenCalledWith('/files/open?path=%2Ftmp%2Fslide.html', { method: 'POST', reportError: false }));
});
