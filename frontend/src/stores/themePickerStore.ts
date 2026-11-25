import { create } from 'zustand';
import type { AssetInfo } from '../api/assets';

export interface ThemePicker {
  projectId: string | null;
  themes: AssetInfo[];
  themeId: string;
  loading: boolean;
  error: boolean;
  applying: boolean;
  load: () => Promise<AssetInfo[] | null>;
  apply: (theme: AssetInfo) => Promise<void>;
}

// The mounted toolbar owns loading and applying; other entry points share it.
export const useThemePickerStore = create<{ picker: ThemePicker | null }>(() => ({ picker: null }));
