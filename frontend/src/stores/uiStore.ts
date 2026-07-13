import { create } from 'zustand';
import { persist } from 'zustand/middleware';

interface UIState {
  leftPanelHidden: boolean;
  rightPanelHidden: boolean;
  themeDensity: 'comfortable' | 'compact';
  activeModeColor: string;

  toggleLeftPanel: () => void;
  toggleRightPanel: () => void;
  setModeColor: (color: string) => void;
}

export const useUIStore = create<UIState>()(
  persist(
    (set) => ({
      leftPanelHidden: false,
      rightPanelHidden: false,
      themeDensity: 'comfortable',
      activeModeColor: 'normal',

      toggleLeftPanel: () => set((state) => ({ leftPanelHidden: !state.leftPanelHidden })),
      toggleRightPanel: () => set((state) => ({ rightPanelHidden: !state.rightPanelHidden })),
      setModeColor: (color) => set({ activeModeColor: color })
    }),
    {
      name: 'ppt-agent-ui-v6',
      partialize: (s) => ({ leftPanelHidden: s.leftPanelHidden, rightPanelHidden: s.rightPanelHidden })
    }
  )
);
