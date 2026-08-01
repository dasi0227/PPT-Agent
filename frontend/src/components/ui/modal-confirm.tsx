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
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription asChild>
            <div className="mt-2 text-text-600">{description}</div>
          </DialogDescription>
        </DialogHeader>
        {error && <div className="text-sm text-danger mt-2">{error}</div>}
        <DialogFooter className="mt-4">
          <button
            type="button"
            onClick={() => onOpenChange(false)}
            disabled={loading}
            className="px-4 py-2 text-sm font-medium rounded-md text-text-600 hover:bg-black/5 transition-colors disabled:opacity-50"
          >
            {cancelLabel}
          </button>
          <button
            type="button"
            onClick={handleConfirm}
            disabled={loading}
            className={cn(
              "px-4 py-2 text-sm font-medium rounded-md transition-colors disabled:opacity-50 flex items-center justify-center min-w-[80px]",
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
