import { useLayoutEffect, useRef, useState } from 'react';
import { X } from 'lucide-react';
import type { DOMSelection } from '../../api/types';
import { IconButton } from '../../components/ui/primitives';

// Match the Runtime number badge: 24px content + 5px padding on each side,
// 24px tall with 5px corners, all measured in the 1920×1080 canvas space.
const BADGE_WIDTH = 34;
const BADGE_HEIGHT = 24;
const BADGE_RADIUS = 5;

export function DraftSelectionControls({ slideId, selections, onRemove }: {
  slideId: string;
  selections: DOMSelection[];
  onRemove: (selectionId: string) => void;
}) {
  const layerRef = useRef<HTMLDivElement>(null);
  const [size, setSize] = useState({ width: 0, height: 0 });

  useLayoutEffect(() => {
    const layer = layerRef.current;
    if (!layer) return;
    // Match Runtime's centered contain-fit canvas. Client dimensions exclude
    // the workspace zoom transform, which already applies to this layer too.
    const measure = () => setSize({ width: layer.clientWidth, height: layer.clientHeight });
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(layer);
    return () => observer.disconnect();
  }, []);

  const scale = Math.min(size.width / 1920, size.height / 1080);
  const width = 1920 * scale;
  const height = 1080 * scale;
  const left = (size.width - width) / 2;
  const top = (size.height - height) / 2;
  const buttonWidth = BADGE_WIDTH * scale;
  const buttonHeight = BADGE_HEIGHT * scale;
  const radius = BADGE_RADIUS * scale;
  const clamp = (value: number, extent: number, controlSize: number) => Math.max(0, Math.min(value, extent - controlSize));

  return (
    <div ref={layerRef} className="pointer-events-none absolute inset-0">
      {scale > 0 && selections
        .filter((selection) => selection.slide_id === slideId && selection.status !== 'page_deleted')
        .map((selection) => {
          return (
            <IconButton
              key={selection.selection_id}
              label={`移除标记 ${selection.marker_no}`}
              className="pointer-events-auto absolute cursor-pointer border-0 bg-danger p-0 text-white hover:bg-danger hover:text-white hover:brightness-95 focus-visible:bg-danger focus-visible:text-white focus-visible:brightness-90"
              style={{
                left: left + clamp((selection.rect.x + selection.rect.width) * scale - buttonWidth, width, buttonWidth),
                top: top + clamp(selection.rect.y * scale - buttonHeight, height, buttonHeight),
                width: buttonWidth,
                height: buttonHeight,
                borderRadius: `${radius}px ${radius}px 0 ${radius}px`,
              }}
              onPointerDown={(event) => event.stopPropagation()}
              onClick={(event) => {
                event.stopPropagation();
                onRemove(selection.selection_id);
              }}
            >
              <X width={13 * scale} height={13 * scale} aria-hidden="true" />
            </IconButton>
          );
        })}
    </div>
  );
}
