import { useRef, useState, type ReactNode } from 'react';
import { X } from 'lucide-react';
import { Dialog, DialogClose, DialogContent, DialogTitle, DialogTrigger } from './dialog';
import { IconButton } from './primitives';

type ImagePreviewProps = {
  name: string;
  src: string;
  className?: string;
} & (
  | { thumbnailSrc: string; children?: never }
  | { thumbnailSrc?: never; children: ReactNode }
);

function PreviewImage({ src, name }: Pick<ImagePreviewProps, 'src' | 'name'>) {
  const [status, setStatus] = useState<'loading' | 'loaded' | 'error'>('loading');

  return (
    <div className="relative grid min-h-32 place-items-center overflow-hidden rounded-md bg-panel-muted">
      {status !== 'loaded' && (
        <p role="status" className="absolute px-4 text-center text-sm text-text-600">
          {status === 'error' ? '图片加载失败，请关闭后重试' : '正在加载图片…'}
        </p>
      )}
      {status !== 'error' && (
        <img
          src={src}
          alt={name}
          onLoad={() => setStatus('loaded')}
          onError={() => setStatus('error')}
          className={`max-h-[calc(90dvh-100px)] max-w-full object-contain ${status === 'loaded' ? '' : 'invisible'}`}
        />
      )}
    </div>
  );
}

export function ImagePreview({ name, thumbnailSrc, src, children, className }: ImagePreviewProps) {
  const [open, setOpen] = useState(false);
  const [failedThumbnail, setFailedThumbnail] = useState<string | null>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);

  if (failedThumbnail === thumbnailSrc) return null;

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <button
          ref={triggerRef}
          type="button"
          aria-label={`预览 ${name}`}
          title="查看原图"
          className={className ?? 'grid h-[38px] w-[38px] shrink-0 cursor-zoom-in place-items-center overflow-hidden rounded-md bg-panel-muted ui-interactive'}
        >
          {thumbnailSrc ? <img
            src={thumbnailSrc}
            alt=""
            loading="lazy"
            onError={() => setFailedThumbnail(thumbnailSrc)}
            className="h-full w-full object-contain"
          /> : children}
        </button>
      </DialogTrigger>
      <DialogContent
        aria-describedby={undefined}
        className="max-h-[90dvh] w-[calc(100vw-32px)] min-w-0 max-w-[960px] gap-3 overflow-hidden p-4"
        onCloseAutoFocus={(event) => {
          event.preventDefault();
          triggerRef.current?.focus();
        }}
      >
        <div className="flex min-w-0 items-center justify-between gap-3">
          <DialogTitle className="min-w-0 truncate text-sm" title={name}>{name}</DialogTitle>
          <DialogClose asChild>
            <IconButton label="关闭图片预览"><X className="h-4 w-4" /></IconButton>
          </DialogClose>
        </div>
        {open && <PreviewImage key={src} src={src} name={name} />}
      </DialogContent>
    </Dialog>
  );
}
