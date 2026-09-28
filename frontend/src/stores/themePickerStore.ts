import { create } from 'zustand';
import type { Theme } from '../api/types';

export interface ThemePicker {
  projectId: string | null;
  themes: Theme[];
  themeId: string;
  loading: boolean;
  error: boolean;
  applying: boolean;
  load: () => Promise<Theme[] | null>;
  apply: (theme: Theme) => Promise<void>;
}

// The mounted toolbar owns loading and applying; other entry points share it.
export const useThemePickerStore = create<{ picker: ThemePicker | null }>(() => ({ picker: null }));
