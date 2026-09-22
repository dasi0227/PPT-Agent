import { useEffect, useRef, useState } from 'react';
import { ChevronDown, Code2, X } from 'lucide-react';
import type { DOMSelection } from '../../api/types';
import { AnchoredPopover, AnchoredPopoverContent, AnchoredPopoverTitle, AnchoredPopoverTrigger } from '../../components/ui/anchored-popover';
import { Button, IconButton } from '../../components/ui/primitives';
import { cn } from '../../lib/utils';

interface Props {
  selection: DOMSelection;
  editing: boolean;
  onEditingChange: (editing: boolean) => void;
  onCommentChange: (comment: string) => void;
  onNavigate: () => void;
  onRemove: () => void;
  onFocusComposer: () => void;
}

export function DOMSelectionReference({ selection, editing, onEditingChange, onCommentChange, onNavigate, onRemove, onFocusComposer }: Props) {
  const anchorRef = useRef<HTMLButtonElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const returnToComposer = useRef(false);
  const composing = useRef(false);
  const [compositionDraft, setCompositionDraft] = useState<string | null>(null);
  const value = compositionDraft ?? selection.comment;
  const saveComment = (comment: string) => onCommentChange(Array.from(comment).slice(0, 500).join(''));
  const finish = () => {
    if (inputRef.current) saveComment(inputRef.current.value);
    returnToComposer.current = true;
    onEditingChange(false);
  };

  useEffect(() => {
    if (!editing) return;
    let frame = 0;
    const onWindowBlur = () => {
      // Pointer/focus events inside the slide iframe do not reach the portal.
      frame = requestAnimationFrame(() => {
        if (document.activeElement instanceof HTMLIFrameElement) {
          if (inputRef.current) onCommentChange(Array.from(inputRef.current.value).slice(0, 500).join(''));
          onEditingChange(false);
        }
      });
    };
    window.addEventListener('blur', onWindowBlur);
    return () => {
      cancelAnimationFrame(frame);
      window.removeEventListener('blur', onWindowBlur);
    };
  }, [editing, onCommentChange, onEditingChange]);

  return (
    <AnchoredPopover open={editing} onOpenChange={(open) => {
      if (open) {
        returnToComposer.current = false;
        setCompositionDraft(null);
        onNavigate();
      } else if (inputRef.current) {
        saveComment(inputRef.current.value);
      }
      onEditingChange(open);
    }}>
      <div className={cn('flex shrink-0 items-center self-center rounded-lg border hover:bg-accent-soft focus-within:bg-accent-soft', editing ? 'border-transparent bg-accent-soft' : 'border-border bg-surface')}>
        <AnchoredPopoverTrigger asChild>
          <button ref={anchorRef} type="button" title={`编辑标记 ${selection.marker_no} 的注释`}
            className="flex h-[34px] items-center gap-1.5 rounded-md bg-transparent px-2 text-xs font-medium">
            <Code2 className="h-[15px] w-[15px] text-accent" strokeWidth={1.75} />
            标记 {selection.marker_no}
          </button>
        </AnchoredPopoverTrigger>
        <IconButton label={`移除标记 ${selection.marker_no}`} onClick={onRemove} className="mr-0.5 h-6 w-6 hover:bg-transparent hover:text-danger focus-visible:bg-transparent focus-visible:text-danger">
          <X className="h-3.5 w-3.5" strokeWidth={1.75} />
        </IconButton>
      </div>
      {editing && (
        <AnchoredPopoverContent anchorRef={anchorRef}
          onEscapeKeyDown={(event) => {
            if (event.isComposing || composing.current) event.preventDefault();
          }}
          onOpenAutoFocus={(event) => {
            event.preventDefault();
            inputRef.current?.focus({ preventScroll: true });
            const length = inputRef.current?.value.length ?? 0;
            inputRef.current?.setSelectionRange(length, length);
          }}
          onCloseAutoFocus={(event) => {
            if (!returnToComposer.current) return;
            event.preventDefault();
            returnToComposer.current = false;
            onFocusComposer();
          }}>
          <div className="flex items-center gap-1.5 px-3 pb-1 pt-2.5">
            <Code2 className="h-3.5 w-3.5 text-accent" strokeWidth={1.75} />
            <AnchoredPopoverTitle className="text-xs font-medium">
              标记 {selection.marker_no} · 注释
            </AnchoredPopoverTitle>
            <IconButton label="收起注释，保留内容" onClick={finish} className="ml-auto h-6 w-6 focus-visible:bg-panel-muted">
              <ChevronDown className="h-3.5 w-3.5" strokeWidth={1.75} />
            </IconButton>
          </div>
          <textarea ref={inputRef} value={value} aria-label={`标记 ${selection.marker_no} 注释`} placeholder="希望这里如何调整？（可选）"
            onChange={(event) => {
              if (composing.current) setCompositionDraft(event.target.value);
              else saveComment(event.target.value);
            }}
            onCompositionStart={() => { composing.current = true; }}
            onCompositionEnd={(event) => {
              composing.current = false;
              setCompositionDraft(null);
              saveComment(event.currentTarget.value);
            }}
            onBlur={(event) => {
              composing.current = false;
              setCompositionDraft(null);
              saveComment(event.currentTarget.value);
            }}
            onKeyDown={(event) => {
              if (event.nativeEvent.isComposing || composing.current) return;
              if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) {
                event.preventDefault();
                event.stopPropagation();
                finish();
              }
            }}
            className="mx-3 mt-1 block h-20 w-[calc(100%-24px)] resize-none overflow-y-auto overscroll-contain rounded border-0 bg-transparent px-2.5 py-2 text-[13px] leading-relaxed outline-none placeholder:text-text-400 focus:bg-panel"
          />
          <div className="flex items-center justify-between px-3 pb-2.5 pt-1">
            <span className="text-[10px] text-text-400">{Array.from(value).length} / 500</span>
            <Button type="button" variant="primary" onClick={finish} className="h-6 px-2.5 text-[11px] focus-visible:bg-accent-soft">完成</Button>
          </div>
        </AnchoredPopoverContent>
      )}
    </AnchoredPopover>
  );
}
