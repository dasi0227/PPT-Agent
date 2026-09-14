import { isMac } from '../../lib/platform';

interface NextInputSuggestionsPanelProps {
  items: string[];
  onSelect: (value: string) => void;
}

export function NextInputSuggestionsPanel({ items, onSelect }: NextInputSuggestionsPanelProps) {
  const modifier = isMac() ? 'Option' : 'Alt';
  return (
    <div className="px-3 pb-1 pt-3" aria-label="下一步输入建议">
      <p className="mb-1.5 text-[11px] font-medium leading-4 text-text-600">
        按「{modifier} + 序号」选择或直接输入下一步需求
      </p>
      <div className="space-y-0.5">
        {items.map((item, index) => (
          <button
            key={`${index}:${item}`}
            type="button"
            onClick={() => onSelect(item)}
            className="group flex w-full items-start gap-2 rounded-md px-1.5 py-1 text-left text-sm leading-5 text-text-600 transition-colors hover:bg-panel-muted hover:text-text-900 focus-visible:bg-accent-soft focus-visible:text-text-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-accent"
            aria-label={`${index + 1}. ${item}，按 ${modifier} 加 ${index + 1} 快捷填入`}
          >
            <span className="mt-px w-3.5 shrink-0 text-right text-[11px] font-semibold tabular-nums text-text-400 group-hover:text-accent" aria-hidden="true">
              {index + 1}.
            </span>
            <span className="min-w-0 whitespace-normal break-words">{item}</span>
          </button>
        ))}
      </div>
    </div>
  );
}
