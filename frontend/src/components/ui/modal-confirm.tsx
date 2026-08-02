import * as React from "react"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "./dialog"
import { cn } from "../../lib/utils"

interface ConfirmModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description: React.ReactNode;
  confirmLabel?: string;
  cancelLabel?: string;
  variant?: 'default' | 'danger';
  onConfirm: () => void | Promise<void>;
}

function emphasizeDescription(description: React.ReactNode): React.ReactNode {
  if (typeof description !== 'string') return description;

  return description.split(/(「[^」]+」|不可撤销)/g).map((part, index) => {
    if (!part) return null;
    if (part === '不可撤销') {
      return <strong key={index} className="font-semibold text-danger">不可撤销</strong>;
    }
    if (/^「[^」]+」$/.test(part)) {
      return (
        <span key={index} className="font-semibold text-text-900 underline decoration-accent/70 decoration-2 underline-offset-4">
          {part}
        </span>
      );
    }
    return part;
  });
}

export function ConfirmModal({
  open,
  onOpenChange,
  title,
  description,
  confirmLabel = '确认',
  cancelLabel = '取消',
  variant = 'default',
  onConfirm,
}: ConfirmModalProps) {
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  const handleConfirm = async () => {
    try {
      setLoading(true);
      setError(null);
      await onConfirm();
      onOpenChange(false);
    } catch (e) {
      setError('无法完成，请稍后重试');
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="!gap-0 sm:max-w-[440px] sm:min-w-0">
        <DialogHeader className="!space-y-0">
          <DialogTitle>{title}</DialogTitle>
        </DialogHeader>
        <DialogDescription asChild>
          <div className="mt-4 text-[15px] leading-6 text-text-600">{emphasizeDescription(description)}</div>
        </DialogDescription>
        {error && <div className="text-sm text-danger mt-2">{error}</div>}
        <DialogFooter className="mt-8">
          <button
            type="button"
            onClick={() => onOpenChange(false)}
            disabled={loading}
            className="inline-flex h-9 min-w-[72px] items-center justify-center rounded-md border border-border bg-surface px-3 text-sm font-medium text-text-600 transition-colors hover:bg-panel-muted hover:text-text-900 disabled:opacity-50"
          >
            {cancelLabel}
          </button>
          <button
            type="button"
            onClick={handleConfirm}
            disabled={loading}
            className={cn(
              "inline-flex h-9 min-w-[72px] items-center justify-center rounded-md px-3 text-sm font-medium transition-colors disabled:opacity-50",
              variant === 'danger' 
                ? "bg-danger text-white hover:bg-danger/90"
                : "bg-accent text-white hover:bg-accent/90"
            )}
          >
            {loading ? "..." : confirmLabel}
          </button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
