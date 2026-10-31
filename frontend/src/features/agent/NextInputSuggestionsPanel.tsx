import { useShortcutStore } from '../../stores/shortcutStore';
import { shortcutLabel } from '../../lib/shortcuts';

interface NextInputSuggestionsPanelProps {
  items: string[];
  pressedIndex: number | null;
  onSelect: (value: string) => void;
}

export function NextInputSuggestionsPanel({ items, pressedIndex, onSelect }: NextInputSuggestionsPanelProps) {
  const bindings = useShortcutStore(state => state.bindings);
  return (
    <div className="px-4 py-3 text-sm leading-5 text-text-400" aria-label="下一步输入建议">
      <p className="mb-2">
        按 「Option + 数字」选择建议或直接输入请求
      </p>
      <div className="space-y-1.5">
        {items.map((item, index) => (
          <button
            key={`${index}:${item}`}
            type="button"
            onMouseDown={(event) => event.preventDefault()}
            onClick={() => onSelect(item)}
            data-highlighted={pressedIndex === index ? '' : undefined}
            className="pointer-events-auto flex w-full items-start gap-2 rounded-sm text-left text-sm leading-5 text-text-400 ui-interactive focus-visible:outline-none"
            title={shortcutLabel(bindings[`composer.suggestion${index + 1}`])}
            aria-label={`${index + 1}. ${item}，按 ${shortcutLabel(bindings[`composer.suggestion${index + 1}`])} 快捷填入`}
          >
            <span className="w-3.5 shrink-0 text-right tabular-nums" aria-hidden="true">
              {index + 1}.
            </span>
            <span className="min-w-0 whitespace-normal break-words">{item}</span>
          </button>
        ))}
      </div>
    </div>
  );
}
