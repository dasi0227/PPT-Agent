import * as React from "react"
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "./dialog"

interface FormModalProps<T> {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  initialValue: T;
  validate: (value: T) => string | null;
  onSubmit: (value: T) => Promise<void>;
  renderField: (value: T, setValue: (v: T) => void, error: string | null) => React.ReactNode;
  confirmLabel?: string;
  cancelLabel?: string;
}

export function FormModal<T>({
  open,
  onOpenChange,
  title,
  initialValue,
  validate,
  onSubmit,
  renderField,
  confirmLabel = '确认',
  cancelLabel = '取消',
}: FormModalProps<T>) {
  const [value, setValue] = React.useState<T>(initialValue);
  const [loading, setLoading] = React.useState(false);
  const [submitError, setSubmitError] = React.useState<string | null>(null);

  // Reset state when opened
  React.useEffect(() => {
    if (open) {
      setValue(initialValue);
      setSubmitError(null);
    }
  }, [open, initialValue]);

  const validationError = validate(value);
  const canSubmit = !validationError && !loading;

  const handleSubmit = async (e?: React.FormEvent) => {
    e?.preventDefault();
    if (!canSubmit) return;

    try {
      setLoading(true);
      setSubmitError(null);
      await onSubmit(value);
      onOpenChange(false);
    } catch (e) {
      setSubmitError('重命名失败，请稍后重试');
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="mt-4">
          {renderField(value, setValue, validationError)}
          {submitError && <div className="text-sm text-mode-error mt-2">{submitError}</div>}
          <DialogFooter className="mt-6">
            <button
              type="button"
              onClick={() => onOpenChange(false)}
              disabled={loading}
              className="px-4 py-2 text-sm font-medium rounded-md text-text-600 hover:bg-black/5 transition-colors disabled:opacity-50"
            >
              {cancelLabel}
            </button>
            <button
              type="submit"
              disabled={!canSubmit}
              className="px-4 py-2 text-sm font-medium rounded-md bg-mode-normal text-white hover:bg-mode-normal/90 transition-colors disabled:opacity-50 flex items-center justify-center min-w-[80px]"
            >
              {loading ? "..." : confirmLabel}
            </button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
