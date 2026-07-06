import { create } from 'zustand';
import { InteractionMode, SubMode } from '../features/agent/modeMapping';

interface ComposerState {
  interactionMode: InteractionMode;
  subMode: SubMode;
  userTouchedMode: boolean;   // 用户是否手动切过（关掉智能默认）
  focusNonce: number;         // 递增触发输入框聚焦

  setInteractionMode: (mode: InteractionMode, opts?: { userTouched?: boolean }) => void;
  setSubMode: (subMode: SubMode) => void;
  applySmartDefault: (mode: InteractionMode) => void;   // 仅当未手动切过时生效
  resetTouch: () => void;                               // 切换 project 时重置会话记忆
  requestOutlineFocus: () => void;                      // EmptyState 引导：锁 Outline + 聚焦
}

export const useComposerStore = create<ComposerState>((set) => ({
  interactionMode: 'outline',
  subMode: 'normal',
  userTouchedMode: false,
  focusNonce: 0,

  setInteractionMode: (mode, opts) =>
    set({ interactionMode: mode, userTouchedMode: opts?.userTouched ?? true }),

  setSubMode: (subMode) => set({ subMode }),

  applySmartDefault: (mode) =>
    set((state) => (state.userTouchedMode ? {} : { interactionMode: mode })),

  resetTouch: () => set({ userTouchedMode: false, subMode: 'normal' }),

  requestOutlineFocus: () =>
    set((state) => ({
      interactionMode: 'outline',
      userTouchedMode: true,
      focusNonce: state.focusNonce + 1,
    })),
}));
