import { useRef, useState } from 'react';
import { ArrowLeft, Check, ChevronDown, ChevronRight, Crosshair } from 'lucide-react';
import type { ScopeSelectionKind } from '../../api/types';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuTrigger } from '../../components/ui/dropdown-menu';

export interface ScopePageOption { id: string; ordinal: number; title: string }
export interface ScopeSectionOption { id: string; title: string; pageCount: number }

const pageModes: Array<{ value: ScopeSelectionKind; label: string }> = [
  { value: 'current_page', label: '当前页' },
  { value: 'all_pages', label: '全部页' },
  { value: 'custom_pages', label: '自选页' },
  { value: 'custom_sections', label: '自选章' },
];

export function scopeSelectionLabel(value: ScopeSelectionKind): string {
  return pageModes.find((item) => item.value === value)?.label ?? '当前页';
}

interface TargetSelectorProps {
  selection: ScopeSelectionKind;
  selectedSlideIds: string[];
  selectedSectionIds: string[];
  pages: ScopePageOption[];
  sections: ScopeSectionOption[];
  onSelectionChange: (value: ScopeSelectionKind) => void;
  onToggleSlide: (id: string) => void;
  onToggleSection: (id: string) => void;
  disabled?: boolean;
  emptyProject?: boolean;
}

export function TargetSelector({
  selection, selectedSlideIds, selectedSectionIds, pages, sections,
  onSelectionChange, onToggleSlide, onToggleSection, disabled, emptyProject = false,
}: TargetSelectorProps) {
  const [open, setOpen] = useState(false);
  const [detail, setDetail] = useState<'custom_pages' | 'custom_sections' | null>(null);
  const pageRef = useRef<HTMLDivElement>(null);
  const sectionRef = useRef<HTMLDivElement>(null);
  const backRef = useRef<HTMLButtonElement>(null);
  const label = scopeSelectionLabel(selection);
  const items = detail === 'custom_pages'
    ? pages.map((page) => ({ id: page.id, title: page.title || '未命名页面', ordinal: `第 ${page.ordinal} 页`, checked: selectedSlideIds.includes(page.id) }))
    : sections.map((section, index) => ({ id: section.id, title: section.title || '未命名章节', ordinal: `第 ${index + 1} 章`, checked: selectedSectionIds.includes(section.id) }));
  const goBack = () => {
    const ref = detail === 'custom_pages' ? pageRef : sectionRef;
    setDetail(null);
    requestAnimationFrame(() => ref.current?.focus());
  };

  return (
    <DropdownMenu open={open} onOpenChange={(value) => { setOpen(value); if (!value) setDetail(null); }}>
      <DropdownMenuTrigger asChild>
        <button type="button" aria-label={`范围：${label}`} title={label} disabled={disabled} className="composer-context-trigger composer-target-button">
          <Crosshair className="h-3.5 w-3.5 shrink-0 text-text-600" strokeWidth={1.75} />
          <span className="min-w-0 truncate">{label}</span>
          <ChevronDown className="composer-context-chevron" strokeWidth={1.75} aria-hidden="true" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent side="top" align="end" sideOffset={8} avoidCollisions collisionPadding={12}
        className={`${detail ? 'w-[352px]' : 'w-52'} rounded-xl p-2`}
        onEscapeKeyDown={(event) => { if (detail) { event.preventDefault(); goBack(); } }}>
        {detail ? (
          <>
            <header className="mb-1 flex h-8 items-center gap-2 px-1">
              <button ref={backRef} type="button" aria-label="返回范围选择" onClick={goBack}
                className="flex h-7 w-7 items-center justify-center rounded-md text-text-600 outline-none hover:bg-accent-soft hover:text-accent focus-visible:bg-accent-soft">
                <ArrowLeft className="h-4 w-4" strokeWidth={1.75} />
              </button>
              <span className="text-xs text-text-900">{scopeSelectionLabel(detail)}</span>
              <span className="ml-auto text-[11px] tabular-nums text-text-600">已选 {detail === 'custom_pages' ? selectedSlideIds.length : selectedSectionIds.length}</span>
            </header>
            <div role="listbox" aria-label={detail === 'custom_pages' ? '选择页面' : '选择章节'} aria-multiselectable="true"
              className="max-h-[min(280px,calc(var(--radix-dropdown-menu-content-available-height)-64px))] overflow-y-auto overscroll-contain [scrollbar-color:var(--color-border)_transparent] [scrollbar-width:thin]">
              {items.length === 0 ? <div className="px-2 py-5 text-center text-xs text-text-600">暂无可选内容</div> : items.map((item) => (
                <DropdownMenuItem key={item.id} role="option" aria-selected={item.checked}
                  onSelect={(event) => { event.preventDefault(); if (detail === 'custom_pages') onToggleSlide(item.id); else onToggleSection(item.id); }}
                  className={`flex min-h-9 items-center gap-2 rounded-md px-2 text-xs ${item.checked ? 'bg-accent-soft text-accent' : 'text-text-800'}`}>
                  <span className="shrink-0 whitespace-nowrap tabular-nums text-text-500">{item.ordinal}</span>
                  <span className="min-w-0 flex-1 truncate" title={item.title}>{item.title}</span>
                  <span aria-hidden="true" className={`flex h-4 w-4 shrink-0 items-center justify-center rounded border ${item.checked ? 'border-accent/30 bg-accent-soft text-accent' : 'border-border bg-surface text-transparent'}`}>
                    <Check className="h-3 w-3" strokeWidth={1.75} />
                  </span>
                </DropdownMenuItem>
              ))}
            </div>
          </>
        ) : (
          <>
            <DropdownMenuLabel className="px-2 pb-2 text-[11px] font-normal text-text-600">范围选择</DropdownMenuLabel>
            {pageModes.map((item) => {
              const custom = item.value === 'custom_pages' || item.value === 'custom_sections';
              return (
                <DropdownMenuItem key={item.value} ref={item.value === 'custom_pages' ? pageRef : item.value === 'custom_sections' ? sectionRef : undefined}
                  role="menuitemradio" aria-checked={item.value === selection} disabled={emptyProject && item.value !== 'all_pages'}
                  onSelect={(event) => {
                    onSelectionChange(item.value);
                    if (item.value === 'custom_pages' || item.value === 'custom_sections') {
                      event.preventDefault(); setDetail(item.value);
                      requestAnimationFrame(() => backRef.current?.focus());
                    }
                  }}
                  className={`flex min-h-9 items-center gap-2 rounded-md px-2 text-xs ${item.value === selection ? 'bg-accent-soft text-accent' : 'text-text-700'}`}>
                  <span className="flex-1">{item.label}</span>
                  {item.value === selection && <Check className="h-3.5 w-3.5" strokeWidth={1.75} aria-hidden="true" />}
                  {custom && <ChevronRight className="h-3.5 w-3.5" strokeWidth={1.75} aria-hidden="true" />}
                </DropdownMenuItem>
              );
            })}
          </>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
