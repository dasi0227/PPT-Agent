import React, { useState } from 'react';
import { AlertTriangle, BarChart3 } from 'lucide-react';
import { Slide, SlideContent } from '../../api/types';
import { SlidePatch } from '../../api/slides';
import { cn } from '../../lib/utils';

interface OutlineCardProps {
  slide: Slide;
  editable: boolean;
  onPatch: (patch: SlidePatch) => void;
  dirty: boolean;
  compact?: boolean;
}

// 从 slide.content 回退到 slide 元数据，保证即使无 slide.json 也可渲染。
function resolveContent(slide: Slide): SlideContent {
  return slide.content ?? { layout: slide.layout, title: slide.title };
}

export const OutlineCard: React.FC<OutlineCardProps> = ({ slide, editable, onPatch, dirty, compact }) => {
  const content = resolveContent(slide);
  const [title, setTitle] = useState(content.title ?? '');
  const [bulletsText, setBulletsText] = useState((content.bullets ?? []).join('\n'));

  // Sync state when slide content changes from outside (e.g. SSE update)
  React.useEffect(() => {
    setTitle(content.title ?? '');
    setBulletsText((content.bullets ?? []).join('\n'));
  }, [content.title, content.bullets]);

  // isEmpty：user 尚未编辑任何有效大纲字段。用于展示占位而非空白页（#6）。
  const isEmpty =
    !content.title
    && !(content.bullets && content.bullets.length > 0)
    && !content.subtitle
    && !content.content_intent;

  const layoutBadge = content.layout || slide.layout;

  const commitTitle = () => {
    const next = title.trim();
    if (next !== (content.title ?? '')) onPatch({ title: next });
  };

  const commitBullets = () => {
    const next = bulletsText.split('\n').map((b) => b.trim()).filter(Boolean);
    const cur = content.bullets ?? [];
    if (next.join('\u0000') !== cur.join('\u0000')) onPatch({ bullets: next });
  };

  // 空态 · compact：网格缩略图中的 Slide N + 未编辑大纲。
  if (isEmpty && compact) {
    return (
      <div className="w-full h-full flex flex-col items-center justify-center bg-white p-3 relative">
        <span className="absolute top-3 left-3 px-2 py-0.5 rounded-full bg-background text-[10px] text-text-400 border border-border uppercase">
          {layoutBadge}
        </span>
        <div className="text-text-400 text-xs">Slide {(slide.order ?? slide.idx) + 1}</div>
        <div className="text-text-400 text-[10px] mt-1">未编辑大纲</div>
      </div>
    );
  }

  // 空态 · 主区：给出显式的编辑引导。
  if (isEmpty && !compact) {
    return (
      <div className="relative w-full aspect-video bg-white ring-1 ring-border rounded-md shadow-sm flex flex-col items-center justify-center p-8">
        <span className="absolute top-3 left-3 px-2 py-0.5 rounded-full bg-background text-[10px] text-text-400 border border-border uppercase">
          {layoutBadge}
        </span>
        <h1 className="text-3xl text-text-400 font-semibold">未命名</h1>
        <p className="text-text-400 text-sm mt-3">使用右侧对话或点击标题开始编辑</p>
      </div>
    );
  }

  return (
    <div
      className={cn(
        'relative w-full aspect-video bg-white ring-1 ring-border rounded-md shadow-sm overflow-hidden flex flex-col',
        compact ? 'p-3' : 'p-8'
      )}
    >
      {/* layout 徽标 */}
      <div className="absolute top-3 left-3 flex items-center gap-2">
        <span className="px-2 py-0.5 rounded-full bg-background text-[10px] text-text-400 border border-border uppercase tracking-wide">
          {content.layout || slide.layout}
        </span>
      </div>

      {/* 脏标记：大纲已改、产物待更新 */}
      {dirty && (
        <div
          className="absolute top-3 right-3 flex items-center gap-1 text-amber-600 text-xs font-medium"
          title="大纲已改，待更新"
        >
          <AlertTriangle className={compact ? 'w-3 h-3' : 'w-3.5 h-3.5'} />
          待更新
        </div>
      )}

      <div className={cn('flex-1 flex flex-col min-h-0', compact ? 'mt-5' : 'mt-8')}>
        {/* 标题 */}
        {editable ? (
          <input
            className={cn(
              'w-full bg-transparent font-semibold text-text-900 outline-none border-b border-transparent focus:border-border',
              compact ? 'text-sm' : 'text-3xl'
            )}
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            onBlur={commitTitle}
            aria-label="slide-title"
          />
        ) : (
          <h1 className={cn('font-semibold text-text-900 truncate', compact ? 'text-sm' : 'text-3xl')}>
            {content.title || '未命名'}
          </h1>
        )}

        {/* 副标题 */}
        {content.subtitle && (
          <p className={cn('text-text-600 mt-1', compact ? 'text-[10px] truncate' : 'text-lg')}>{content.subtitle}</p>
        )}

        {/* content_intent */}
        {content.content_intent && !compact && (
          <p className="text-text-400 text-sm mt-2 italic">{content.content_intent}</p>
        )}

        {/* bullets */}
        {editable ? (
          <textarea
            className={cn(
              'mt-4 flex-1 w-full bg-transparent text-text-600 outline-none resize-none border border-transparent focus:border-border rounded p-1',
              compact ? 'text-[10px]' : 'text-base'
            )}
            value={bulletsText}
            onChange={(e) => setBulletsText(e.target.value)}
            onBlur={commitBullets}
            aria-label="slide-bullets"
            placeholder="每行一个要点"
          />
        ) : (
          (content.bullets && content.bullets.length > 0) && (
            <ul className={cn('mt-4 space-y-1 text-text-600 list-disc list-inside', compact ? 'text-[10px]' : 'text-base')}>
              {content.bullets.map((b, i) => (
                <li key={i} className="truncate">{b}</li>
              ))}
            </ul>
          )
        )}

        {/* chart_intent 占位 */}
        {content.chart_intent && (
          <div className={cn('mt-3 flex items-center gap-2 text-text-400', compact ? 'text-[10px]' : 'text-sm')}>
            <BarChart3 className={compact ? 'w-3 h-3' : 'w-4 h-4'} />
            <span>图表：{content.chart_intent.type}{content.chart_intent.data_hint ? ` · ${content.chart_intent.data_hint}` : ''}</span>
          </div>
        )}

        {/* steps */}
        {typeof content.steps === 'number' && content.steps > 0 && !compact && (
          <div className="mt-2 text-text-400 text-sm">步骤数：{content.steps}</div>
        )}
      </div>
    </div>
  );
};
