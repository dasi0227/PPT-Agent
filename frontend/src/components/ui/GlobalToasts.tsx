import { useCallback, useEffect, useRef, useState } from 'react';
import { CircleCheck, CircleX, Info, TriangleAlert, X } from 'lucide-react';
import { type ToastItem, useToastStore } from '../../stores/toastStore';
import { cn } from '../../lib/utils';

const DISPLAY_DURATION_MS = 5_000;
const EXIT_DURATION_MS = 220;

function ToastIcon({ tone }: { tone: ToastItem['tone'] }) {
  const props = { className: 'global-toast-icon', 'aria-hidden': true as const };
  if (tone === 'success') return <CircleCheck {...props} />;
  if (tone === 'warning') return <TriangleAlert {...props} />;
  if (tone === 'error') return <CircleX {...props} />;
  return <Info {...props} />;
}

function GlobalToast({ toast }: { toast: ToastItem }) {
  const removeToast = useToastStore((state) => state.removeToast);
  const [leaving, setLeaving] = useState(false);
  const leavingRef = useRef(false);
  const exitTimerRef = useRef<number>();

  const dismiss = useCallback(() => {
    if (leavingRef.current) return;
    leavingRef.current = true;
    setLeaving(true);
    exitTimerRef.current = window.setTimeout(() => removeToast(toast.id), EXIT_DURATION_MS);
  }, [removeToast, toast.id]);

  useEffect(() => {
    const timer = window.setTimeout(dismiss, DISPLAY_DURATION_MS);
    return () => {
      window.clearTimeout(timer);
      if (exitTimerRef.current !== undefined) window.clearTimeout(exitTimerRef.current);
    };
  }, [dismiss]);

  return (
    <article
      role={toast.tone === 'error' ? 'alert' : 'status'}
      data-tone={toast.tone}
      className={cn(
        'global-toast',
        `global-toast-${toast.tone}`,
        leaving && 'global-toast-leaving',
      )}
    >
      <span className="global-toast-timer" aria-hidden="true" />
      <ToastIcon tone={toast.tone} />
      <span className="global-toast-message">{toast.message}</span>
      <button
        type="button"
        className="global-toast-close"
        aria-label="关闭通知"
        onClick={dismiss}
      >
        <X className="h-3.5 w-3.5" aria-hidden="true" />
      </button>
    </article>
  );
}

export function GlobalToasts() {
  const toasts = useToastStore((state) => state.toasts);

  return (
    <section className="global-toast-viewport" aria-label="系统通知">
      {toasts.map((toast) => <GlobalToast key={toast.id} toast={toast} />)}
    </section>
  );
}
