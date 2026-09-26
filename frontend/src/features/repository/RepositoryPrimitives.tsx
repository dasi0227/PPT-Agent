import { FileOpenButton } from '../../components/ui/FileOpenButton';
import { type ReactNode, useEffect, useState } from 'react';
import { AlertCircle, ChevronRight, ExternalLink, Pause, Pencil, Search, Trash2, type LucideIcon } from 'lucide-react';
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
    <header className="flex h-16 shrink-0 items-center gap-4 border-b border-border bg-surface px-5 md:px-6">
      <h1 className="shrink-0 text-lg font-bold tracking-[-0.01em] text-text-900">{title}</h1>
      <label className="group flex h-9 w-[224px] min-w-0 items-center gap-2 rounded-lg border border-border bg-panel px-3 text-text-400 shadow-[inset_0_1px_0_rgba(255,255,255,0.7)] transition-colors focus-within:bg-surface">
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
    <FileOpenButton
      url={href}
      className="grid h-6 w-6 shrink-0 place-items-center rounded-md text-text-400 transition-colors ui-interactive focus-visible:outline-none"
      label="查看文件"
    >
      <ExternalLink className="h-3.5 w-3.5" strokeWidth={1.75} />
    </FileOpenButton>
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
      <div className="scrollbar-none flex h-12 shrink-0 items-center gap-1 overflow-x-auto border-b border-border px-3">
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
      aria-pressed={active}
      className={cn(
        'h-7 shrink-0 rounded-md px-2.5 text-xs font-semibold transition-all active:translate-y-px focus-visible:outline-none',
        active
          ? 'ui-selected'
          : 'text-text-600 ui-interactive',
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
  descriptionLines = 2,
  icon: Icon,
  onClick,
}: {
  active: boolean;
  disabled?: boolean;
  name: ReactNode;
  description: string;
  descriptionLines?: 2 | 3;
  icon: LucideIcon;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      className={cn(
        'group mb-1 grid min-h-[66px] w-full grid-cols-[58px_minmax(0,1fr)_18px] items-center gap-2.5 rounded-lg border p-2 text-left transition-all active:translate-y-px focus-visible:outline-none',
        active
          ? 'border-accent/30 ui-selected'
          : 'border-transparent ui-interactive',
        disabled && 'text-text-400',
      )}
    >
      <span
        className={cn(
          'grid h-9 w-9 place-items-center justify-self-center rounded-full',
          disabled ? 'bg-panel-muted text-text-400' : 'bg-success-soft text-success',
        )}
        aria-hidden="true"
      >
        {disabled ? <Pause className="h-4 w-4" strokeWidth={1.75} /> : <Icon className="h-4 w-4" strokeWidth={1.75} />}
      </span>
      <span className="min-w-0">
        <span className={cn('block min-w-0 truncate text-[13px] font-bold', disabled ? 'text-text-400' : active ? 'text-selected-foreground' : 'text-text-900')}>
          {name}
        </span>
        <span className={cn('mt-0.5 text-[11px] leading-4 text-text-600', descriptionLines === 3 ? 'line-clamp-3' : 'line-clamp-2')}>{description}</span>
      </span>
      <ChevronRight
        className={cn(
          'h-3.5 w-3.5 text-text-400 transition-transform group-hover:translate-x-0.5',
          active && 'text-selected-foreground',
        )}
        strokeWidth={1.75}
      />
    </button>
  );
}

export function RepositoryDetail({
  active = true,
  label,
  title,
  description,
  openUrl,
  properties,
  actions,
  children,
  contentClassName,
  deleteNoun,
  onEdit,
  onDelete,
}: {
  active?: boolean;
  label: string;
  title: string;
  description?: string;
  openUrl?: string;
  properties?: ReactNode;
  actions?: ReactNode;
  children: ReactNode;
  contentClassName?: string;
  deleteNoun: string;
  onEdit: () => void;
  onDelete: () => void | Promise<void>;
}) {
  const [deleteOpen, setDeleteOpen] = useState(false);
  useEffect(() => { if (!active) setDeleteOpen(false); }, [active]);

  return (
    <section className="flex min-h-[420px] min-w-0 flex-col bg-surface md:min-h-0" aria-label={label}>
      <header className="grid min-h-[128px] shrink-0 grid-cols-1 items-start gap-3 border-b border-border bg-surface px-5 pb-3.5 pt-5 lg:grid-cols-[minmax(0,1fr)_auto] lg:gap-x-6 lg:gap-y-1.5">
        <div className="min-w-0">
          <div className="flex min-w-0 items-center gap-1">
            <h2 className="truncate text-base font-bold leading-6 text-text-900">{title}</h2>
            <div className="flex shrink-0 items-center gap-0.5">
              <button
                type="button"
                onClick={onEdit}
                className="grid h-6 w-6 shrink-0 place-items-center rounded-md text-text-400 transition-colors ui-interactive active:translate-y-px focus-visible:outline-none"
                title={`编辑${deleteNoun}`}
                aria-label={`编辑${title}`}
              >
                <Pencil className="h-3.5 w-3.5" strokeWidth={1.75} />
              </button>
              <button
                type="button"
                onClick={() => setDeleteOpen(true)}
                className="grid h-6 w-6 shrink-0 place-items-center rounded-md text-text-400 transition-colors ui-danger active:translate-y-px focus-visible:outline-none"
                title={`删除${deleteNoun}`}
                aria-label={`删除${title}`}
              >
                <Trash2 className="h-3.5 w-3.5" strokeWidth={1.75} />
              </button>
              <RepositoryFileLink href={openUrl} />
            </div>
          </div>
          {properties && <div className="mt-2.5 min-w-0">{properties}</div>}
        </div>
        {actions && <div className="shrink-0">{actions}</div>}
        {description && (
          <p className="line-clamp-2 min-w-0 text-xs font-medium leading-[18px] text-text-600 lg:col-span-2">
            {description}
          </p>
        )}
      </header>
      <div className={cn('min-h-0 flex-1 overflow-auto bg-canvas/70', contentClassName)}>{children}</div>
      <ConfirmModal
        open={active && deleteOpen}
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
    <div className="scrollbar-none flex w-full min-w-0 max-w-full flex-nowrap gap-1.5 overflow-x-auto pb-1" aria-label="标签">
      {tags.map((tag) => (
        <span key={tag} className="shrink-0 rounded-md border border-border bg-surface px-1.5 py-0.5 text-[11px] font-medium text-text-600">
          {tag}
        </span>
      ))}
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
          aria-pressed={value === option.value}
          className={cn(
            'h-7 rounded-md px-3 text-xs font-semibold transition-all active:translate-y-px focus-visible:outline-none',
            value === option.value
              ? 'ui-selected'
              : 'text-text-600 ui-interactive',
          )}
        >
          {option.label}
        </button>
      ))}
    </div>
  );
}
