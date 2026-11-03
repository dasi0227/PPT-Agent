import { cn } from '../../lib/utils';

const illustrations = {
  document: {
    src: '/illustrations/empty-document-tray.png',
    width: 1254,
    height: 1254,
    className: 'w-28',
  },
  slides: {
    src: '/illustrations/empty-page-stack.png',
    width: 1334,
    height: 1179,
    className: 'w-56 sm:w-64',
  },
  outline: {
    src: '/illustrations/empty-outline.png',
    width: 1254,
    height: 1254,
    className: 'w-40',
  },
} as const;

interface IllustratedEmptyStateProps {
  illustration: keyof typeof illustrations;
  label?: string;
  className?: string;
}

export function IllustratedEmptyState({ illustration, label, className }: IllustratedEmptyStateProps) {
  const image = illustrations[illustration];

  return (
    <div className={cn('flex max-w-full flex-col items-center gap-2 text-center text-sm leading-5 text-text-400', className)}>
      <img
        src={image.src}
        width={image.width}
        height={image.height}
        alt=""
        aria-hidden="true"
        draggable={false}
        decoding="async"
        className={cn('pointer-events-none h-auto max-w-full shrink-0 select-none object-contain', image.className)}
      />
      {label && <p>{label}</p>}
    </div>
  );
}
