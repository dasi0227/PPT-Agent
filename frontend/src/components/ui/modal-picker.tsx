import * as React from "react"
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "./dialog"
import { Search } from "lucide-react"

interface PickerModalProps<T> {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  leadingAction?: React.ReactNode;
  confirmationLabel?: string;
  items: T[];
  keyOf: (item: T) => string;
  searchOf: (item: T) => string;
  renderItem: (item: T) => React.ReactNode;
  onPick: (item: T) => void;
  emptyState?: React.ReactNode;
}

export function PickerModal<T>({
  open,
  onOpenChange,
  title,
  leadingAction,
  confirmationLabel,
  items,
  keyOf,
  searchOf,
  renderItem,
  onPick,
  emptyState,
}: PickerModalProps<T>) {
  const [search, setSearch] = React.useState("");
  const [activeIndex, setActiveIndex] = React.useState(0);
  const [selectedKey, setSelectedKey] = React.useState<string | null>(null);

  const filteredItems = React.useMemo(() => {
    if (!search.trim()) return items;
    const q = search.toLowerCase();
    return items.filter(item => searchOf(item).toLowerCase().includes(q));
  }, [items, search, searchOf]);

  React.useEffect(() => {
    if (open) {
      setSearch("");
      setActiveIndex(0);
      setSelectedKey(null);
    }
  }, [open]);

  React.useEffect(() => {
    setActiveIndex(0);
    setSelectedKey(null);
  }, [search]);

  const selectedItem = selectedKey === null
    ? undefined
    : items.find((item) => keyOf(item) === selectedKey);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (filteredItems.length === 0) return;
    
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setActiveIndex(prev => (prev + 1) % filteredItems.length);
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setActiveIndex(prev => (prev - 1 + filteredItems.length) % filteredItems.length);
    } else if (e.key === 'Enter') {
      e.preventDefault();
      const item = filteredItems[activeIndex];
      if (!item) return;
      if (confirmationLabel) {
        setSelectedKey(keyOf(item));
      } else {
        onPick(item);
      }
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="p-0 gap-0 overflow-hidden flex flex-col max-h-[80vh]">
        <DialogHeader className="border-b border-border px-5 pb-5 pt-5">
          <div className="flex items-center gap-2 pr-8">
            {leadingAction}
            <DialogTitle>{title}</DialogTitle>
          </div>
          <div className="relative mt-7">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-text-400" />
            <input
              type="text"
              value={search}
              onChange={e => setSearch(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder="搜索项目"
              className="h-11 w-full rounded-lg border border-border bg-panel pl-10 pr-4 text-sm text-text-900 placeholder:text-text-400 transition-colors focus:border-border-strong focus:outline-none focus-visible:outline-none focus-visible:ring-0 focus-visible:ring-offset-0"
              autoFocus
            />
          </div>
        </DialogHeader>
        <div className="flex-1 overflow-y-auto p-2">
          {filteredItems.length === 0 ? (
            emptyState || <div className="p-4 text-center text-sm text-text-400">无结果</div>
          ) : (
            <div className="flex flex-col gap-1">
              {filteredItems.map((item, index) => {
                const itemKey = keyOf(item);
                const isActive = confirmationLabel ? itemKey === selectedKey : index === activeIndex;
                return (
                  <button
                    key={itemKey}
                    type="button"
                    aria-pressed={confirmationLabel ? isActive : undefined}
                    className={`text-left p-3 rounded-md transition-colors ${isActive ? 'bg-black/5 ring-1 ring-border-strong' : 'hover:bg-black/5'}`}
                    onClick={() => {
                      if (confirmationLabel) {
                        setActiveIndex(index);
                        setSelectedKey(itemKey);
                      } else {
                        onPick(item);
                      }
                    }}
                    onMouseEnter={() => setActiveIndex(index)}
                  >
                    {renderItem(item)}
                  </button>
                );
              })}
            </div>
          )}
        </div>
        {confirmationLabel && (
          <DialogFooter className="shrink-0 border-t border-border px-5 py-4">
            <button
              type="button"
              disabled={!selectedItem}
              onClick={() => selectedItem && onPick(selectedItem)}
              className="inline-flex h-9 items-center justify-center rounded-md bg-accent px-4 text-sm font-medium text-white transition-colors hover:bg-accent/90 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {confirmationLabel}
            </button>
          </DialogFooter>
        )}
      </DialogContent>
    </Dialog>
  );
}
