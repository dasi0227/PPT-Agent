import { isMac } from '../../lib/platform';

interface NextInputSuggestionsPanelProps {
  items: string[];
  onSelect: (value: string) => void;
}

export function NextInputSuggestionsPanel({ items, onSelect }: NextInputSuggestionsPanelProps) {
  const modifier = isMac() ? 'Option' : 'Alt';
  return (
    <div className="px-4 py-3 text-sm leading-5 text-text-400" aria-label="下一步输入建议">
      <p className="mb-2">
        按「{modifier} + 序号」选择或直接输入下一步需求
      </p>
      <div className="space-y-1.5">
        {items.map((item, index) => (
          <button
            key={`${index}:${item}`}
            type="button"
            onMouseDown={(event) => event.preventDefault()}
            onClick={() => onSelect(item)}
            className="pointer-events-auto flex max-w-full items-start gap-2 rounded-sm text-left text-sm leading-5 text-text-400 hover:text-text-900 focus-visible:bg-accent-soft focus-visible:text-text-900 focus-visible:outline-none"
            aria-label={`${index + 1}. ${item}，按 ${modifier} 加 ${index + 1} 快捷填入`}
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
