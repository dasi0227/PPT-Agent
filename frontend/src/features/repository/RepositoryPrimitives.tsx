import { type ReactNode, useState } from 'react';
import { AlertCircle, Check, ChevronRight, ExternalLink, Pencil, Search, Trash2, X } from 'lucide-react';
import { ConfirmModal } from '../../components/ui/modal-confirm';
import { cn } from '../../lib/utils';
import { Skeleton } from '../../components/ui/primitives';

export function RepositoryPageHeader({
  title,
  query,
  onQueryChange,
  searchLabel,
}: {
  title: string;
  query: string;
  onQueryChange: (value: string) => void;
  searchLabel: string;
}) {
  return (
    <header className="flex h-16 shrink-0 items-center justify-between gap-4 border-b border-border bg-surface px-5 md:px-6">
      <h1 className="text-lg font-bold tracking-[-0.01em] text-text-900">{title}</h1>
      <label className="group flex h-9 w-full max-w-[280px] items-center gap-2 rounded-lg border border-border bg-panel px-3 text-text-400 shadow-[inset_0_1px_0_rgba(255,255,255,0.7)] transition-colors focus-within:border-accent focus-within:bg-surface focus-within:ring-2 focus-within:ring-accent/10">
        <Search className="h-4 w-4 shrink-0 transition-colors group-focus-within:text-accent" strokeWidth={1.75} />
        <input
          value={query}
          onChange={(event) => onQueryChange(event.target.value)}
          className="min-w-0 flex-1 bg-transparent text-[13px] text-text-900 outline-none placeholder:text-text-400"
          placeholder={searchLabel}
          aria-label={searchLabel}
        />
      </label>
    </header>
  );
}

export function RepositoryFileLink({ href }: { href?: string }) {
  if (!href) return null;
  return (
    <a
      href={href}
      className="grid h-6 w-6 shrink-0 place-items-center rounded-md text-text-400 transition-colors hover:bg-panel-muted hover:text-text-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
      title="查看文件"
      aria-label="查看文件"
    >
      <ExternalLink className="h-3.5 w-3.5" strokeWidth={1.75} />
    </a>
  );
}

export function RepositoryWorkspace({ children }: { children: ReactNode }) {
  return (
    <div
      data-repository-workspace
      className="grid min-h-0 flex-1 grid-cols-1 overflow-hidden md:grid-cols-[320px_minmax(0,1fr)]"
    >
      {children}
    </div>
  );
}

export function RepositoryCatalog({
  label,
  controls,
  children,
  footer,
}: {
  label: string;
  controls: ReactNode;
  children: ReactNode;
  footer?: ReactNode;
}) {
  return (
    <aside
      className="flex max-h-[340px] min-h-0 flex-col border-b border-border bg-panel md:max-h-none md:border-b-0 md:border-r"
      aria-label={label}
    >
      <div className="flex h-12 shrink-0 items-center gap-1 overflow-x-auto border-b border-border px-3">
        {controls}
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto p-2">{children}</div>
      {footer && <footer className="shrink-0 border-t border-border p-2.5">{footer}</footer>}
    </aside>
  );
}

export function RepositoryFilterButton({
  active,
  children,
  onClick,
}: {
  active: boolean;
  children: ReactNode;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        'h-7 shrink-0 rounded-md px-2.5 text-xs font-semibold transition-all active:translate-y-px focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-1',
        active
          ? 'bg-surface text-accent shadow-[0_1px_3px_rgba(51,65,85,0.12)] ring-1 ring-border'
          : 'text-text-600 hover:bg-panel-muted hover:text-text-900',
      )}
    >
      {children}
    </button>
  );
}

export function RepositoryDirectoryItem({
  active,
  disabled = false,
  name,
  description,
  visual,
  visualClassName,
  visualBare = false,
  onClick,
}: {
  active: boolean;
  disabled?: boolean;
  name: ReactNode;
  description: string;
  visual: ReactNode;
  visualClassName?: string;
  visualBare?: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        'group mb-1 grid min-h-[66px] w-full grid-cols-[58px_minmax(0,1fr)_18px] items-center gap-2.5 rounded-lg border p-2 text-left transition-all active:translate-y-px focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-inset',
        active
          ? 'border-accent/30 bg-accent-soft shadow-[0_2px_8px_rgba(47,103,246,0.07)]'
          : 'border-transparent hover:border-border hover:bg-surface',
        disabled && 'text-text-400',
      )}
    >
      <span
        className={cn(
          'grid h-[42px] w-[58px] place-items-center overflow-hidden',
          !visualBare && 'rounded-md border border-border bg-surface',
          visualClassName,
        )}
        aria-hidden="true"
      >
        {visual}
      </span>
      <span className="min-w-0">
        <span className={cn('block min-w-0 truncate text-[13px] font-bold', disabled ? 'text-text-400' : 'text-text-900')}>
          {name}
        </span>
        <span className="mt-0.5 block line-clamp-2 text-[11px] leading-4 text-text-600">{description}</span>
      </span>
      <ChevronRight
        className={cn(
          'h-3.5 w-3.5 text-text-400 transition-transform group-hover:translate-x-0.5',
          active && 'text-accent',
        )}
        strokeWidth={1.75}
      />
    </button>
  );
}

export function RepositoryDetail({
  label,
  title,
  description,
  openUrl,
  properties,
  actions,
  children,
  contentClassName,
  deleteNoun,
  onDelete,
}: {
  label: string;
  title: string;
  description?: string;
  openUrl?: string;
  properties?: ReactNode;
  actions?: ReactNode;
  children: ReactNode;
  contentClassName?: string;
  deleteNoun: string;
  onDelete: () => void | Promise<void>;
}) {
  const [deleteOpen, setDeleteOpen] = useState(false);

  return (
    <section className="flex min-h-[420px] min-w-0 flex-col bg-surface md:min-h-0" aria-label={label}>
      <header className="flex min-h-[128px] shrink-0 flex-col items-start justify-between gap-3 border-b border-border bg-surface px-5 pb-3.5 pt-5 lg:flex-row lg:gap-6">
        <div className="min-w-0">
          <div className="flex min-w-0 items-center gap-1">
            <h2 className="truncate text-base font-bold leading-6 text-text-900">{title}</h2>
            <div className="flex shrink-0 items-center gap-0.5">
              <RepositoryFileLink href={openUrl} />
              <button
                type="button"
                onClick={() => setDeleteOpen(true)}
                className="grid h-6 w-6 shrink-0 place-items-center rounded-md text-text-400 transition-colors hover:bg-danger-soft hover:text-danger active:translate-y-px focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-danger"
                title={`删除${deleteNoun}`}
                aria-label={`删除${title}`}
              >
                <Trash2 className="h-3.5 w-3.5" strokeWidth={1.75} />
              </button>
            </div>
          </div>
          {description && (
            <p className="mt-1.5 line-clamp-2 max-w-3xl text-xs font-medium leading-[18px] text-text-600">
              {description}
            </p>
          )}
          {properties && <div className="mt-5 min-w-0">{properties}</div>}
        </div>
        {actions && <div className="shrink-0">{actions}</div>}
      </header>
      <div className={cn('min-h-0 flex-1 overflow-auto bg-canvas/70', contentClassName)}>{children}</div>
      <ConfirmModal
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title={`删除${deleteNoun}`}
        description={`确定删除「${title}」吗？该操作不可撤销。`}
        confirmLabel="删除"
        variant="danger"
        onConfirm={onDelete}
      />
    </section>
  );
}

export function RepositoryTagList({ tags }: { tags: string[] }) {
  if (tags.length === 0) return null;
  return (
    <div className="flex w-full min-w-0 max-w-full flex-nowrap gap-1.5 overflow-x-auto pb-1" aria-label="标签">
      {tags.map((tag) => (
        <span key={tag} className="shrink-0 rounded-md border border-border bg-surface px-1.5 py-0.5 text-[11px] font-medium text-text-600">
          {tag}
        </span>
      ))}
    </div>
  );
}

export function RepositoryTagEditor<T extends string>({
  value,
  options,
  onSave,
}: {
  value: T[];
  options: Array<{ value: T; label: string }>;
  onSave: (value: T[]) => Promise<void>;
}) {
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState<T[]>(value);
  const [saving, setSaving] = useState(false);

  const begin = () => {
    setDraft(value);
    setOpen(true);
  };
  const toggle = (tag: T) => {
    setDraft((current) => current.includes(tag)
      ? current.filter((value) => value !== tag)
      : current.length < 2 ? [...current, tag] : current);
  };
  const save = async () => {
    setSaving(true);
    try {
      await onSave(draft);
      setOpen(false);
    } catch {
      // The parent reports the save error and the editor stays open for retry.
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="relative">
      <button
        type="button"
        onClick={begin}
        className="grid h-7 w-7 place-items-center rounded-md border border-border bg-surface text-text-600 hover:bg-panel-muted hover:text-text-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
        title="编辑标签"
        aria-label="编辑标签"
      >
        <Pencil className="h-3.5 w-3.5" strokeWidth={1.75} />
      </button>
      {open && (
        <div className="absolute right-0 top-9 z-20 w-[220px] rounded-lg border border-border bg-surface p-2 shadow-pop" role="dialog" aria-label="编辑标签">
          <div className="flex flex-wrap gap-1.5">
            {options.map((option) => {
              const selected = draft.includes(option.value);
              return (
                <button
                  key={option.value}
                  type="button"
                  aria-pressed={selected}
                  disabled={!selected && draft.length >= 2}
                  onClick={() => toggle(option.value)}
                  className={cn(
                    'h-7 rounded-md border px-2 text-xs font-semibold disabled:cursor-not-allowed disabled:opacity-40',
                    selected ? 'border-accent/30 bg-accent-soft text-accent' : 'border-border bg-surface text-text-600 hover:bg-panel-muted',
                  )}
                >
                  {option.label}
                </button>
              );
            })}
          </div>
          <div className="mt-2 flex justify-end gap-1.5 border-t border-border pt-2">
            <button
              type="button"
              onClick={() => setOpen(false)}
              disabled={saving}
              className="grid h-7 w-7 place-items-center rounded-md text-text-500 hover:bg-panel-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
              title="取消"
              aria-label="取消编辑标签"
            >
              <X className="h-3.5 w-3.5" />
            </button>
            <button
              type="button"
              onClick={() => void save()}
              disabled={saving}
              className="grid h-7 w-7 place-items-center rounded-md bg-accent text-white hover:bg-accent/90 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-1 disabled:opacity-50"
              title="保存"
              aria-label="保存标签"
            >
              <Check className="h-3.5 w-3.5" />
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

export function RepositoryState({
  text,
  error = false,
  className,
}: {
  text: string;
  error?: boolean;
  className?: string;
}) {
  return (
    <div
      role={error ? 'alert' : 'status'}
      className={cn('grid min-h-44 flex-1 place-items-center px-6 text-center', className)}
    >
      <div className={cn('flex items-center gap-2 text-sm font-medium', error ? 'text-danger' : 'text-text-600')}>
        {error && <AlertCircle className="h-4 w-4" strokeWidth={1.75} />}
        <span>{text}</span>
      </div>
    </div>
  );
}

export function RepositoryLoading({ aside = false }: { aside?: boolean }) {
  return (
    <div role="status" aria-label="正在加载" className={cn('grid min-h-0 flex-1', aside ? 'grid-cols-1 md:grid-cols-[320px_minmax(0,1fr)]' : 'grid-cols-1')}>
      {aside && (
        <div className="space-y-3 border-b border-border bg-panel p-3 md:border-b-0 md:border-r">
          {Array.from({ length: 5 }).map((_, index) => (
            <div key={index} className="flex gap-3 rounded-lg px-2 py-2.5">
              <Skeleton className="h-[42px] w-[58px] shrink-0" />
              <div className="flex-1 space-y-2 py-1">
                <Skeleton className="h-4 w-2/3" />
                <Skeleton className="h-3 w-full" />
              </div>
            </div>
          ))}
        </div>
      )}
      <div className="flex min-h-0 items-center justify-center bg-canvas/60 p-8">
        <div className="aspect-video w-full max-w-4xl space-y-4 rounded-lg border border-border bg-surface p-8 shadow-canvas">
          <Skeleton className="h-5 w-28" />
          <Skeleton className="h-10 w-2/3" />
          <Skeleton className="h-4 w-1/2" />
          <Skeleton className="mt-8 h-28 w-full" />
        </div>
      </div>
    </div>
  );
}

export function SegmentedControl<T extends string>({
  value,
  options,
  onChange,
  label,
}: {
  value: T;
  options: Array<{ value: T; label: string }>;
  onChange: (value: T) => void;
  label: string;
}) {
  return (
    <div className="flex shrink-0 rounded-lg bg-panel-muted p-0.5" aria-label={label}>
      {options.map((option) => (
        <button
          key={option.value}
          type="button"
          onClick={() => onChange(option.value)}
          className={cn(
            'h-7 rounded-md px-3 text-xs font-semibold transition-all active:translate-y-px focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-1',
            value === option.value
              ? 'bg-surface text-text-900 shadow-[0_1px_3px_rgba(51,65,85,0.14)] ring-1 ring-border/80'
              : 'text-text-600 hover:text-text-900',
          )}
        >
          {option.label}
        </button>
      ))}
    </div>
  );
}
