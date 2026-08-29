import { useCallback, useEffect, useRef, useState } from 'react';
import { X } from 'lucide-react';
import { type ErrorToast, useToastStore } from '../../stores/toastStore';
import { cn } from '../../lib/utils';

const DISPLAY_DURATION_MS = 5_000;
const EXIT_DURATION_MS = 220;

function GlobalErrorToast({ toast }: { toast: ErrorToast }) {
  const removeError = useToastStore((state) => state.removeError);
  const [leaving, setLeaving] = useState(false);
  const leavingRef = useRef(false);
  const exitTimerRef = useRef<number>();

  const dismiss = useCallback(() => {
    if (leavingRef.current) return;
    leavingRef.current = true;
    setLeaving(true);
    exitTimerRef.current = window.setTimeout(() => removeError(toast.id), EXIT_DURATION_MS);
  }, [removeError, toast.id]);

  useEffect(() => {
    const timer = window.setTimeout(dismiss, DISPLAY_DURATION_MS);
    return () => {
      window.clearTimeout(timer);
      if (exitTimerRef.current !== undefined) window.clearTimeout(exitTimerRef.current);
    };
  }, [dismiss]);

  return (
    <article
      role="alert"
      className={cn('global-error-toast', leaving && 'global-error-toast-leaving')}
    >
      <span className="global-error-toast-timer" aria-hidden="true" />
      <span className="global-error-toast-message">{toast.message}</span>
      <button
        type="button"
        className="global-error-toast-close"
        aria-label="关闭错误通知"
        onClick={dismiss}
      >
        <X className="h-3.5 w-3.5" aria-hidden="true" />
      </button>
    </article>
  );
}

export function GlobalErrorToasts() {
  const errors = useToastStore((state) => state.errors);

  return (
    <section
      className="global-error-toast-viewport"
      aria-label="系统错误通知"
      aria-live="assertive"
      aria-atomic="false"
    >
      {errors.map((toast) => <GlobalErrorToast key={toast.id} toast={toast} />)}
    </section>
  );
}
