import { useLayoutEffect, useRef, useState } from 'react';
import { X } from 'lucide-react';
import type { DOMSelection } from '../../api/types';
import { IconButton } from '../../components/ui/primitives';

const BUTTON_SIZE = 24;
const GAP = 4;

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
  const clamp = (value: number, extent: number) => Math.max(GAP, Math.min(value, extent - BUTTON_SIZE - GAP));

  return (
    <div ref={layerRef} className="pointer-events-none absolute inset-0">
      {width >= BUTTON_SIZE + GAP * 2 && height >= BUTTON_SIZE + GAP * 2 && selections
        .filter((selection) => selection.slide_id === slideId && selection.status !== 'page_deleted')
        .map((selection) => {
          const y = selection.rect.y * scale;
          // Small selections need their number badge's space above the box.
          const fitsAbove = y >= BUTTON_SIZE + GAP && selection.rect.width * scale >= BUTTON_SIZE * 2 + GAP;
          const buttonTop = fitsAbove ? y - BUTTON_SIZE - GAP : y + GAP;
          return (
            <IconButton
              key={selection.selection_id}
              label={`移除标记 ${selection.marker_no}`}
              className="pointer-events-auto absolute h-6 w-6 cursor-pointer border border-border bg-surface hover:bg-danger-soft hover:text-danger focus-visible:bg-danger-soft focus-visible:text-danger"
              style={{
                left: left + clamp((selection.rect.x + selection.rect.width) * scale - BUTTON_SIZE, width),
                top: top + clamp(buttonTop, height),
              }}
              onPointerDown={(event) => event.stopPropagation()}
              onClick={(event) => {
                event.stopPropagation();
                onRemove(selection.selection_id);
              }}
            >
              <X className="h-3.5 w-3.5" aria-hidden="true" />
            </IconButton>
          );
        })}
    </div>
  );
}
