import { Moon, Sun } from 'lucide-react';
import { useAppearanceStore } from '../../stores/appearanceStore';
import { IconButton } from './primitives';

export function ColorModeToggle({ className }: { className?: string }) {
  const { colorMode, toggleColorMode } = useAppearanceStore();
  const dark = colorMode === 'dark';
  return (
    <IconButton
      label={dark ? '切换到亮色模式' : '切换到暗色模式'}
      expandableLabel={dark ? '亮色' : '暗色'}
      onClick={toggleColorMode}
      className={className}
    >
      {dark ? <Sun size={16} /> : <Moon size={16} />}
    </IconButton>
  );
}
