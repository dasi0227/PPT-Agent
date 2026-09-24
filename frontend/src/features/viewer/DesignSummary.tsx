import type { DecorationType, Design } from '../../api/types';
import { decorationPlacementLabel, decorationTypeLabel } from './semanticLabels';
import { DocumentSection } from './DocumentSection';

const decorationOrder: DecorationType[] = ['page_number', 'section_title', 'deck_title', 'key_message'];

export function DesignSummary({ design }: { design: Design }) {
  return (
    <section className="rounded-xl border border-border bg-surface p-5 shadow-sm">
      <h3 className="font-semibold text-text-900">全局视觉规范</h3>
      <div className="mt-4"><DesignDetails design={design} compact /></div>
    </section>
  );
}

export function DesignDetails({ design, compact = false }: { design: Design; compact?: boolean }) {
  const direction = design.direction.trim();
  const directionLabel = direction || '暂无视觉方向';

  return (
    <div className={`${compact ? 'space-y-5' : 'space-y-7'} text-sm font-normal leading-7 text-text-700`}>
      <DocumentSection title="视觉方向" compact={compact}>
        <p className="whitespace-pre-wrap break-words">{directionLabel}</p>
      </DocumentSection>
      <DocumentSection title="排版偏好" compact={compact}>
        {design.layout_preferences.length ? (
          <ul className="space-y-1">
            {design.layout_preferences.map((preference, index) => <li key={index} className="whitespace-pre-wrap break-words">{preference}</li>)}
          </ul>
        ) : <p>暂无排版偏好</p>}
      </DocumentSection>
      <DocumentSection title="页面装饰" compact={compact}>
        <div className="space-y-4">
          {decorationOrder.map((type) => {
            const placement = design.decorations[type];
            return (
              <section key={type}>
                <h3 className="mb-1 text-sm font-semibold leading-6 text-text-900">{decorationTypeLabel(type)}</h3>
                <p>{decorationPlacementLabel(placement)}</p>
              </section>
            );
          })}
        </div>
      </DocumentSection>
    </div>
  );
}
