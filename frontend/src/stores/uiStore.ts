import { create } from 'zustand';
import { persist } from 'zustand/middleware';

interface UIState {
  leftPanelHidden: boolean;
  rightPanelHidden: boolean;

  toggleLeftPanel: () => void;
  toggleRightPanel: () => void;
  showRightPanel: () => void;
}

export const useUIStore = create<UIState>()(
  persist(
    (set) => ({
      leftPanelHidden: false,
      rightPanelHidden: false,

      toggleLeftPanel: () => set((state) => ({ leftPanelHidden: !state.leftPanelHidden })),
      toggleRightPanel: () => set((state) => ({ rightPanelHidden: !state.rightPanelHidden })),
      showRightPanel: () => set({ rightPanelHidden: false })
    }),
    {
      name: 'ppt-agent-ui-v6',
      partialize: (s) => ({ leftPanelHidden: s.leftPanelHidden, rightPanelHidden: s.rightPanelHidden })
    }
  )
);
