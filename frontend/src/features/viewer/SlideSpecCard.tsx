import { Eye, MessageSquareText } from 'lucide-react';
import type { MaterializationState, SlideSpec } from '../../api/types';
import { MaterializationBadge } from './MaterializationBadge';

export function SlideSpecCard({ spec, state, compact = false }: {
  spec: SlideSpec;
  state: MaterializationState;
  compact?: boolean;
}) {
  return (
    <article className={`h-full w-full overflow-y-auto rounded-xl border border-border bg-surface ${compact ? 'p-3' : 'p-7'} shadow-sm`}>
      <div className="flex items-center justify-between gap-2">
        <span className="rounded bg-accent-soft px-2 py-1 text-[10px] font-semibold text-accent">
          {spec.role}
        </span>
        <MaterializationBadge state={state} />
      </div>
      <h2 className={`${compact ? 'mt-2 text-sm' : 'mt-5 text-2xl'} font-semibold text-text-900`}>{spec.title}</h2>
      <div className={`${compact ? 'mt-2 text-[10px]' : 'mt-4 text-base'} flex gap-2 rounded-lg bg-accent-soft/70 p-3 text-text-900`}>
        <MessageSquareText className="h-4 w-4 shrink-0 text-accent" />
        <strong>{spec.key_message}</strong>
      </div>
      {!compact && (
        <>
          <p className="mt-5 text-sm text-text-600">{spec.content.summary}</p>
          {spec.content.points.length > 0 && (
            <ul className="mt-3 list-disc space-y-1 pl-5 text-sm text-text-600">
              {spec.content.points.map((point) => <li key={point}>{point}</li>)}
            </ul>
          )}
          <div className="mt-5 flex gap-2 border-t border-border pt-4 text-sm text-text-600">
            <Eye className="h-4 w-4 shrink-0 text-accent" />
            <div><strong>{spec.visual_intent.archetype}</strong><p className="mt-1">{spec.visual_intent.description}</p></div>
          </div>
        </>
      )}
    </article>
  );
}
