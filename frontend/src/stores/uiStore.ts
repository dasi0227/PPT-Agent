import { create } from 'zustand';

interface UIState {
  rightPanelOpen: boolean;
  leftPanelCollapsed: boolean;
  themeDensity: 'comfortable' | 'compact';
  activeModeColor: string;

  toggleRightPanel: () => void;
  toggleLeftPanel: () => void;
  setModeColor: (color: string) => void;
}

export const useUIStore = create<UIState>((set) => ({
  rightPanelOpen: true,
  leftPanelCollapsed: false,
  themeDensity: 'comfortable',
  activeModeColor: 'normal',

  toggleRightPanel: () => set((state) => ({ rightPanelOpen: !state.rightPanelOpen })),
  toggleLeftPanel: () => set((state) => ({ leftPanelCollapsed: !state.leftPanelCollapsed })),
  setModeColor: (color) => set({ activeModeColor: color })
}));
