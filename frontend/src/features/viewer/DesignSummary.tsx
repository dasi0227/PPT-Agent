import type { Design } from '../../api/types';

type ChromeItem = Design['chrome'][number];

const CHROME_TYPE_LABELS: Record<ChromeItem['type'], string> = {
  page_number: '页码',
  section_marker: '章节标记',
  key_message: '核心信息',
  deck_title: '演示标题',
};

const CHROME_PLACEMENT_LABELS: Record<ChromeItem['placement'], string> = {
  'top-left': '左上',
  'top-center': '顶部居中',
  'top-right': '右上',
  'bottom-left': '左下',
  'bottom-center': '底部居中',
  'bottom-right': '右下',
  'left-edge': '左侧边',
  'right-edge': '右侧边',
};

const DENSITY_LABELS: Record<Design['density'], string> = {
  sparse: '宽松',
  medium: '适中',
  dense: '紧凑',
};

function describeChrome(item: ChromeItem): string {
  const type = CHROME_TYPE_LABELS[item.type] ?? item.type;
  const placement = CHROME_PLACEMENT_LABELS[item.placement] ?? item.placement;
  return `${type}（${placement}）`;
}

export function DesignSummary({ design }: { design: Design }) {
  return (
    <section className="rounded-xl border border-border bg-surface p-5 shadow-sm">
      <div className="flex items-center justify-between">
        <h3 className="font-semibold text-text-900">全局视觉规范</h3>
        <span className="text-xs text-text-400">rev {design.revision}</span>
      </div>
      <p className="mt-3 text-xs font-medium uppercase tracking-wide text-text-400">
        {design.theme} · {DENSITY_LABELS[design.density] ?? design.density}
      </p>
      <p className="mt-2 text-sm text-text-600">{design.direction}</p>
      {design.chrome.length > 0 && (
        <p className="mt-2 text-xs text-text-400">
          页面装饰：{design.chrome.map(describeChrome).join(' · ')}
        </p>
      )}
    </section>
  );
}
