import type React from 'react';
import { useRef, useState } from 'react';
import { Check, Crosshair, X } from 'lucide-react';
import type { ScopeObject, ScopeSelectionKind } from '../../api/types';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '../../components/ui/dropdown-menu';

export interface ScopePageOption { id: string; ordinal: number; title: string }
export interface ScopeSectionOption { id: string; title: string; pageCount: number }

const pageModes: Array<{ value: ScopeSelectionKind; label: string }> = [
  { value: 'current_page', label: '当前页' },
  { value: 'all_pages', label: '全部页' },
  { value: 'custom_pages', label: '自选页' },
  { value: 'custom_sections', label: '自选章' },
];

const objectModes: Array<{ value: ScopeObject; label: string }> = [
  { value: 'spec', label: '设计稿' },
  { value: 'html', label: '幻灯片' },
  { value: 'presentation', label: '演示文稿' },
  { value: 'global', label: '全局资源' },
];

export function scopeSelectionLabel(value: ScopeSelectionKind): string {
  return pageModes.find((item) => item.value === value)?.label ?? '当前页';
}

export function scopeObjectLabel(value: ScopeObject): string {
  return objectModes.find((item) => item.value === value)?.label ?? '演示文稿';
}

export function composerScopeLabel(selection: ScopeSelectionKind, object: ScopeObject): string {
  return `${scopeSelectionLabel(selection)} · ${scopeObjectLabel(object)}`;
}

interface TargetSelectorProps {
  object: ScopeObject;
  selection: ScopeSelectionKind;
  selectedSlideIds: string[];
  selectedSectionIds: string[];
  pages: ScopePageOption[];
  sections: ScopeSectionOption[];
  onObjectChange: (value: ScopeObject) => void;
  onSelectionChange: (value: ScopeSelectionKind) => void;
  onToggleSlide: (id: string) => void;
  onToggleSection: (id: string) => void;
  disabled?: boolean;
  locked?: boolean;
}

export const TargetSelector: React.FC<TargetSelectorProps> = ({
  object,
  selection,
  selectedSlideIds,
  selectedSectionIds,
  pages,
  sections,
  onObjectChange,
  onSelectionChange,
  onToggleSlide,
  onToggleSection,
  disabled,
  locked = false,
}) => {
  const [scopeOpen, setScopeOpen] = useState(false);
  const [customOpen, setCustomOpen] = useState(true);
  const customPageTriggerRef = useRef<HTMLDivElement>(null);
  const customSectionTriggerRef = useRef<HTMLDivElement>(null);
  const effectiveSelection = object === 'global' ? 'all_pages' : selection;
  const label = composerScopeLabel(effectiveSelection, object);
  const segmentClass = (active: boolean, unavailable = false) => [
    'flex h-7 min-w-0 flex-1 items-center justify-center rounded-md px-2 text-[11px] font-medium outline-none transition-[color,background-color,box-shadow] focus-visible:ring-2 focus-visible:ring-accent',
    active ? 'bg-surface text-text-900 shadow-sm' : 'text-text-600 hover:bg-surface/70 hover:text-text-900',
    unavailable ? 'pointer-events-none opacity-35' : '',
  ].join(' ');
  const customMode = effectiveSelection === 'custom_pages' || effectiveSelection === 'custom_sections';
  const customItems = effectiveSelection === 'custom_pages'
    ? pages.map((page) => ({ id: page.id, title: page.title || '未命名页面', meta: String(page.ordinal), checked: selectedSlideIds.includes(page.id) }))
    : sections.map((section) => ({ id: section.id, title: section.title || '未命名章节', meta: `${section.pageCount} 页`, checked: selectedSectionIds.includes(section.id) }));
  const closeCustomWindow = () => {
    setCustomOpen(false);
    requestAnimationFrame(() => {
      (effectiveSelection === 'custom_pages' ? customPageTriggerRef : customSectionTriggerRef).current?.focus();
    });
  };

  const trigger = (
    <button
      type="button"
      aria-label={`范围：${label}`}
      disabled={disabled}
      className="composer-target-button inline-flex h-7 min-w-0 max-w-[168px] shrink-0 items-center gap-1.5 rounded-md border border-border bg-transparent px-2 text-[11px] font-medium text-text-700 transition-colors hover:bg-panel-muted hover:text-text-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent disabled:cursor-not-allowed disabled:opacity-45"
    >
      <Crosshair className="h-3.5 w-3.5 shrink-0" strokeWidth={1.75} />
      <span className="composer-target-label min-w-0 truncate">{label}</span>
    </button>
  );

  if (locked) return trigger;

  return (
    <DropdownMenu open={scopeOpen} onOpenChange={(open) => { setScopeOpen(open); if (open && customMode) setCustomOpen(true); }}>
      <DropdownMenuTrigger asChild>{trigger}</DropdownMenuTrigger>
      <DropdownMenuContent
        side="top"
        align="end"
        sideOffset={8}
        className="w-[340px] border-0 bg-transparent p-0 shadow-none"
        onEscapeKeyDown={(event) => {
          if (!customMode || !customOpen) return;
          event.preventDefault();
          closeCustomWindow();
        }}
      >
        {customMode && customOpen && (
          <section className="mb-2 overflow-hidden rounded-xl bg-surface shadow-lg">
            <header className="flex h-9 items-center gap-2 border-b border-border/70 px-3">
              <strong className="text-xs font-semibold text-text-900">{effectiveSelection === 'custom_pages' ? '自选页' : '自选章'}</strong>
              <span className="rounded-full bg-panel-muted px-1.5 py-0.5 text-[10px] tabular-nums text-text-700">{effectiveSelection === 'custom_pages' ? selectedSlideIds.length : selectedSectionIds.length}</span>
              <button type="button" aria-label="收起自选窗口" className="ml-auto flex h-6 w-6 items-center justify-center rounded-md text-text-500 hover:bg-panel-muted hover:text-text-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent" onPointerDown={(event) => event.stopPropagation()} onClick={(event) => { event.preventDefault(); event.stopPropagation(); closeCustomWindow(); }}>
                <X className="h-3.5 w-3.5" />
              </button>
            </header>
            <div className="bg-panel-muted/50 p-1">
              <div role="listbox" aria-label={effectiveSelection === 'custom_pages' ? '选择页面' : '选择章节'} aria-multiselectable="true" className="max-h-[208px] overflow-y-auto overscroll-contain pr-0.5 [scrollbar-color:var(--color-border)_transparent] [scrollbar-width:thin]">
                {customItems.length === 0 ? (
                  <div className="px-2.5 py-5 text-center text-xs text-text-600">暂无可选内容</div>
                ) : customItems.map((item) => (
                  <DropdownMenuItem
                    key={item.id}
                    role="option"
                    aria-selected={item.checked}
                    onSelect={(event) => {
                      event.preventDefault();
                      if (effectiveSelection === 'custom_pages') onToggleSlide(item.id);
                      else onToggleSection(item.id);
                    }}
                    className="flex min-h-9 cursor-pointer items-center gap-2 rounded-md px-2 text-xs text-text-800 focus:bg-surface"
                  >
                    <span className="w-7 shrink-0 text-right tabular-nums text-text-500">{item.meta}</span>
                    <span className="min-w-0 flex-1 truncate">{item.title}</span>
                    <span className={[
                      'flex h-4 w-4 shrink-0 items-center justify-center rounded border transition-colors',
                      item.checked ? 'border-accent bg-accent text-white' : 'border-border bg-surface text-transparent',
                    ].join(' ')} aria-hidden="true">
                      <Check className="h-3 w-3" strokeWidth={2.5} />
                    </span>
                  </DropdownMenuItem>
                ))}
              </div>
            </div>
          </section>
        )}
        <section className="overflow-hidden rounded-xl bg-surface p-2 shadow-lg">
          <header className="mb-1 flex h-7 items-center px-1">
            <strong className="text-xs font-semibold text-text-900">任务范围</strong>
            <button type="button" aria-label="关闭任务范围" className="ml-auto flex h-6 w-6 items-center justify-center rounded-md text-text-500 hover:bg-panel-muted hover:text-text-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent" onSelect={(event) => event.preventDefault()} onClick={() => setScopeOpen(false)}>
              <X className="h-3.5 w-3.5" />
            </button>
          </header>
          <div className="flex items-center gap-2">
            <span className="w-8 shrink-0 text-[11px] font-medium text-text-700">对象</span>
            <div role="radiogroup" aria-label="修改对象" className="grid min-w-0 flex-1 grid-cols-4 gap-1 rounded-lg bg-panel-muted p-1">
              {objectModes.map((item) => (
                <DropdownMenuItem
                  key={item.value}
                  role="radio"
                  aria-checked={item.value === object}
                  aria-label={`修改对象：${item.label}`}
                  onSelect={(event) => { event.preventDefault(); onObjectChange(item.value); }}
                  className={segmentClass(item.value === object)}
                >
                  {item.label}
                </DropdownMenuItem>
              ))}
            </div>
          </div>
          <div className="mt-1.5 flex items-center gap-2">
            <span className="w-8 shrink-0 text-[11px] font-medium text-text-700">页面</span>
            <div role="radiogroup" aria-label="页面范围" className="grid min-w-0 flex-1 grid-cols-4 gap-1 rounded-lg bg-panel-muted p-1">
              {pageModes.map((item) => {
                const unavailable = object === 'global';
                return (
                  <DropdownMenuItem
                    key={item.value}
                    ref={item.value === 'custom_pages' ? customPageTriggerRef : item.value === 'custom_sections' ? customSectionTriggerRef : undefined}
                    role="radio"
                    aria-checked={item.value === effectiveSelection}
                    aria-label={`页面范围：${item.label}`}
                    aria-disabled={unavailable}
                    disabled={unavailable}
                    onSelect={(event) => {
                      event.preventDefault();
                      onSelectionChange(item.value);
                      if (item.value === 'custom_pages' || item.value === 'custom_sections') setCustomOpen(true);
                    }}
                    className={segmentClass(item.value === effectiveSelection, unavailable)}
                  >
                    {item.label}
                  </DropdownMenuItem>
                );
              })}
            </div>
          </div>
        </section>
      </DropdownMenuContent>
    </DropdownMenu>
  );
};
