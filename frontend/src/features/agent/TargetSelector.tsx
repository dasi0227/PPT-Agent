import { useRef, useState } from 'react';
import { ArrowLeft, Check, ChevronDown, ChevronRight, Crosshair } from 'lucide-react';
import type { ScopeSelectionKind } from '../../api/types';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuTrigger } from '../../components/ui/dropdown-menu';

export interface ScopePageOption { id: string; ordinal: number; title: string }
export interface ScopeChapterOption { id: string; title: string }

const pageModes: Array<{ value: ScopeSelectionKind; label: string }> = [
  { value: 'current_page', label: '当前页' },
  { value: 'all_pages', label: '全部页' },
  { value: 'custom_pages', label: '自选页' },
  { value: 'custom_chapters', label: '自选章' },
  { value: 'custom_sections', label: '自选节' },
];

export function scopeSelectionLabel(value: ScopeSelectionKind): string {
  return pageModes.find((item) => item.value === value)?.label ?? '当前页';
}

interface TargetSelectorProps {
  selection: ScopeSelectionKind;
  selectedSlideIds: string[];
  selectedChapterIds: string[];
  selectedSectionIds: string[];
  pages: ScopePageOption[];
  chapters: ScopeChapterOption[];
  sections: { id: string; title: string; ordinal: string }[];
  onSelectionChange: (value: ScopeSelectionKind) => void;
  onToggleSlide: (id: string) => void;
  onToggleChapter: (id: string) => void;
  onToggleSection: (id: string) => void;
  disabled?: boolean;
  emptyProject?: boolean;
}

export function TargetSelector({
  selection, selectedSlideIds, selectedChapterIds, selectedSectionIds, pages, chapters, sections,
  onSelectionChange, onToggleSlide, onToggleChapter, onToggleSection, disabled, emptyProject = false,
}: TargetSelectorProps) {
  const [open, setOpen] = useState(false);
  const [detail, setDetail] = useState<'custom_pages' | 'custom_chapters' | 'custom_sections' | null>(null);
  const pageRef = useRef<HTMLDivElement>(null);
  const chapterRef = useRef<HTMLDivElement>(null);
  const sectionRef = useRef<HTMLDivElement>(null);
  const backRef = useRef<HTMLButtonElement>(null);
  const label = scopeSelectionLabel(selection);
  const choices = {
    custom_pages: { ids: selectedSlideIds, toggle: onToggleSlide, ref: pageRef, label: '选择页面' },
    custom_chapters: { ids: selectedChapterIds, toggle: onToggleChapter, ref: chapterRef, label: '选择章' },
    custom_sections: { ids: selectedSectionIds, toggle: onToggleSection, ref: sectionRef, label: '选择节' },
  };
  const activeChoice = detail && choices[detail];
  const items = detail === 'custom_pages'
    ? pages.map((page) => ({ id: page.id, title: page.title || '未命名页面', ordinal: `第 ${page.ordinal} 页` }))
    : detail === 'custom_chapters'
      ? chapters.map((chapter, index) => ({ id: chapter.id, title: chapter.title || '未命名章', ordinal: `第 ${index + 1} 章` }))
      : sections;
  const goBack = () => {
    const ref = activeChoice?.ref;
    setDetail(null);
    requestAnimationFrame(() => ref?.current?.focus());
  };

  return (
    <DropdownMenu open={open} onOpenChange={(value) => { setOpen(value); if (!value) setDetail(null); }}>
      <DropdownMenuTrigger asChild disabled={disabled}>
        <button type="button" aria-label={`范围：${label}`} title={label} disabled={disabled} className="composer-context-trigger ui-interactive composer-target-button">
          <Crosshair className="h-4 w-4 shrink-0 text-accent" strokeWidth={1.75} />
          <span className="min-w-0 truncate font-medium">{label}</span>
          <ChevronDown className="composer-context-chevron" strokeWidth={1.75} aria-hidden="true" />
        </button>
      </DropdownMenuTrigger>
      {/* Include the context bar's 7px inset and the composer's 1px border. */}
      <DropdownMenuContent side="top" align="end" alignOffset={-8} sideOffset={8} avoidCollisions collisionPadding={12}
        className={`${detail ? 'w-[352px]' : 'w-52'} rounded-[var(--radius-card)] p-2`}
        onEscapeKeyDown={(event) => { if (detail) { event.preventDefault(); goBack(); } }}>
        {detail && activeChoice ? (
          <>
            <header className="mb-1 flex h-8 items-center gap-2 px-1">
              <button ref={backRef} type="button" aria-label="返回范围选择" onClick={goBack}
                className="flex h-7 w-7 items-center justify-center rounded-md text-text-600 outline-none ui-interactive">
                <ArrowLeft className="h-4 w-4" strokeWidth={1.75} />
              </button>
              <span className="text-xs text-text-900">{scopeSelectionLabel(detail)}</span>
              <span className="ml-auto text-[11px] tabular-nums text-text-600">已选 {activeChoice.ids.length}</span>
            </header>
            <div role="listbox" aria-label={activeChoice.label} aria-multiselectable="true"
              className="max-h-[min(280px,calc(var(--radix-dropdown-menu-content-available-height)-64px))] overflow-y-auto overscroll-contain [scrollbar-color:var(--color-border)_transparent] [scrollbar-width:thin]">
              {items.length === 0 ? <div className="px-2 py-5 text-center text-xs text-text-600">暂无可选内容</div> : items.map((item) => (
                <DropdownMenuItem key={item.id} role="option" aria-selected={activeChoice.ids.includes(item.id)}
                  onSelect={(event) => { event.preventDefault(); activeChoice.toggle(item.id); }}
                  className={`flex min-h-9 items-center gap-2 rounded-md px-2 text-xs ${activeChoice.ids.includes(item.id) ? 'ui-selected' : 'text-text-800'}`}>
                  <span className="shrink-0 whitespace-nowrap font-normal tabular-nums text-text-500">{item.ordinal}</span>
                  <span className="min-w-0 flex-1 truncate" title={item.title}>{item.title}</span>
                  <span aria-hidden="true" className={`flex h-4 w-4 shrink-0 items-center justify-center rounded border ${activeChoice.ids.includes(item.id) ? 'border-accent/30 ui-selected' : 'border-border bg-surface text-transparent'}`}>
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
              const custom = item.value === 'current_page' || item.value === 'all_pages' ? null : item.value;
              return (
                <DropdownMenuItem key={item.value} ref={custom ? choices[custom].ref : undefined}
                  role="menuitemradio" aria-checked={item.value === selection} disabled={emptyProject && item.value !== 'all_pages'}
                  onSelect={(event) => {
                    onSelectionChange(item.value);
                    if (custom) {
                      event.preventDefault(); setDetail(custom);
                      requestAnimationFrame(() => backRef.current?.focus());
                    }
                  }}
                  className={`flex min-h-9 items-center gap-2 rounded-md px-2 text-xs ${item.value === selection ? 'ui-selected' : 'text-text-700'}`}>
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
