import { useId, useState, type ReactNode } from 'react';
import { ChevronUp, CornerUpLeft, Ellipsis, File, Folder } from 'lucide-react';
import type { DiffHunk, FieldDiff, PublicTarget, DecorationPlacement, DecorationType, TextDiffRow, OutlineDiffGroup } from '../../api/types';
import { useProjectStore } from '../../stores/projectStore';
import { orderedSlides } from '../deck/selectors';
import { decorationPlacementLabel, decorationTypeLabel, elementTypeLabel, slideContentTypeLabel, slidePurposeLabel } from '../viewer/semanticLabels';
import { decorationTypes } from '../viewer/decorationPositions';
import { LongContent } from './LongContent';
import { openSourceTarget } from './SourceCard';
import { TimelineCardHeader, timelineCardActionClass } from './TimelineCardHeader';
import { ChangeStats } from './ChangeStats';
import { IconButton } from '../../components/ui/primitives';
import { cn } from '../../lib/utils';

const fieldLabels: Record<string, string> = {
  title: '演示标题', language: '演示语言', pages: '演示页数', audience: '演示受众', goal: '演示目标',
  requirements: '内容需求', prohibitions: '内容限制', core: '核心信息', layout: '布局建议',
  purpose: '页面用途', content_type: '正文类型', elements: '内容元素',
};
const fieldOrders: Partial<Record<PublicTarget['part'], readonly string[]>> = {
  manifest: ['title', 'language', 'pages', 'audience', 'goal', 'requirements', 'prohibitions'],
  design: ['demands', ...decorationTypes.map(type => `decorations.${type}`)],
};
function orderedFields(fields: FieldDiff[], part: PublicTarget['part']): FieldDiff[] {
  const order = fieldOrders[part];
  if (!order) return fields;
  const ranks = new Map(order.map((field, index) => [field, index]));
  return [...fields].sort((a, b) => (ranks.get(a.field) ?? order.length) - (ranks.get(b.field) ?? order.length));
}
function fieldLabel(field: FieldDiff, part: PublicTarget['part']): string {
  if (field.label) return field.label;
  if (field.field.startsWith('decorations.')) return `${decorationTypeLabel(field.field.slice(12) as DecorationType)}位置`;
  if (field.field === 'demands' && part === 'design') return '视觉需求';
  return fieldLabels[field.field] ?? (field.field || '内容');
}
function fieldValue(field: FieldDiff, source: string): string {
  const value: unknown = JSON.parse(source);
  if (field.field === 'purpose' && typeof value === 'string') return slidePurposeLabel(value);
  if (field.field === 'content_type' && typeof value === 'string') return slideContentTypeLabel(value);
  if (field.field.startsWith('decorations.') && typeof value === 'string') return decorationPlacementLabel(value as DecorationPlacement | 'none');
  if (value && typeof value === 'object' && !Array.isArray(value) && 'type' in value && typeof value.type === 'string' && 'intent' in value && typeof value.intent === 'string') {
    const element = value as { type: string; intent: string };
    return `${elementTypeLabel(element.type)} · ${element.intent}`;
  }
  if (typeof value === 'string') return value || '""';
  if (typeof value === 'number') return source;
  return JSON.stringify(value) ?? '';
}
const rowClass = (kind: string) => kind === 'added' ? 'bg-success-soft text-success' : kind === 'removed' ? 'bg-danger-soft text-danger' : 'text-text-600';

function OutlineDiff({ groups }: { groups: OutlineDiffGroup[] }) {
  return groups.map((group, index) => <section key={index} className={index > 0 ? 'mt-3 border-t border-border pt-3' : ''}>
    {group.rows.map((row, rowIndex) => <div key={rowIndex} className={`flex min-h-[30px] items-center gap-2 whitespace-nowrap py-[3px] pr-3.5 text-xs ${rowClass(row.kind)}`}
      style={{ paddingLeft: 14 + row.depth * 22 }}>
      <span className="w-4 shrink-0 select-none text-center font-mono" aria-label={row.kind === 'added' ? '新增' : row.kind === 'removed' ? '删除' : undefined}>
        {row.kind === 'added' ? '+' : row.kind === 'removed' ? '−' : ''}
      </span>
      {row.node === 'purpose' ? <span className="text-[11px] opacity-90">目的</span>
        : row.node === 'page' ? <File size={14} strokeWidth={1.75} className="shrink-0" aria-hidden="true" />
          : <Folder size={14} strokeWidth={1.75} className="shrink-0" aria-hidden="true" />}
      {row.order && <span className="min-w-[18px] text-[11px] tabular-nums opacity-80">{row.order}</span>}
      <span className={row.node === 'chapter' || row.node === 'subchapter' ? 'font-medium' : undefined}>{row.title}</span>
    </div>)}
  </section>);
}

function TextDiffRows({ rows }: { rows: TextDiffRow[] }) {
  return rows.map((row, index) => <div key={index} className={`grid min-h-[23px] grid-cols-[38px_38px_23px_max-content] whitespace-pre pr-4 font-mono text-xs leading-[23px] ${rowClass(row.kind)}`}>
    <span className={`sticky left-0 select-none pr-2 text-right text-[11px] tabular-nums text-text-500 ${row.kind === 'added' ? 'bg-success-soft' : row.kind === 'removed' ? 'bg-danger-soft' : 'bg-timeline-card'}`}>{row.old_line ?? ''}</span>
    <span className={`sticky left-[38px] select-none border-r border-border pr-2 text-right text-[11px] tabular-nums text-text-500 ${row.kind === 'added' ? 'bg-success-soft' : row.kind === 'removed' ? 'bg-danger-soft' : 'bg-timeline-card'}`}>{row.new_line ?? ''}</span>
    <span className="select-none text-center">{row.kind === 'added' ? '+' : row.kind === 'removed' ? '−' : ' '}</span><code>{row.text}</code>
  </div>);
}

function TextDiffHunk({ hunk, separated }: { hunk: DiffHunk; separated: boolean }) {
  const [expanded, setExpanded] = useState(false);
  const contextId = useId();
  const context = hunk.context_before ?? [];
  return <section>
    {context.length > 0 ? <>
      <button type="button" aria-label={expanded ? '收起未修改内容' : '展开未修改内容'} aria-expanded={expanded} aria-controls={contextId}
        className="flex w-full border-y border-border bg-panel text-text-500 ui-interactive" onClick={event => { event.stopPropagation(); setExpanded(value => !value); }}>
        <span className="sticky left-0 flex h-7 w-[99px] shrink-0 items-center justify-center bg-inherit">
          {expanded ? <ChevronUp size={18} strokeWidth={1.75} aria-hidden="true" /> : <Ellipsis size={18} strokeWidth={1.75} aria-hidden="true" />}
        </span>
      </button>
      <div id={contextId}>{expanded && <TextDiffRows rows={context} />}</div>
    </> : separated && <div className="h-px bg-border" />}
    <TextDiffRows rows={hunk.rows} />
  </section>;
}

export function ChangePreviewButton({ target, iconOnly = false, label = '预览', className }: {
  target: PublicTarget; iconOnly?: boolean; label?: string; className?: string;
}) {
  const projectId = useProjectStore(state => state.activeProjectId);
  const snapshot = useProjectStore(state => projectId ? state.contentByProjectId[projectId] : undefined);
  const exists = !!snapshot && (target.type === 'deck' || orderedSlides(snapshot).some(slide => slide.id === target.slide_id));
  const canPreview = !!projectId && exists && target.diff?.status !== 'deleted';
  if (target.type === 'file') return null;
  const icon = <CornerUpLeft className="h-3.5 w-3.5" strokeWidth={1.75} aria-hidden="true" />;
  return iconOnly
    ? <IconButton label={label} disabled={!canPreview} className={cn('h-7 w-7 rounded', className)} onClick={() => openSourceTarget(target)}>{icon}</IconButton>
    : <button type="button" disabled={!canPreview} className={cn(timelineCardActionClass, className)} onClick={() => openSourceTarget(target)}>{icon}<span>预览</span></button>;
}

export function ChangeDiffCard({ target, previewEnabled = true, footer, variant = 'card' }: {
  target: PublicTarget; previewEnabled?: boolean; footer?: ReactNode; variant?: 'card' | 'inline';
}) {
  const diff = target.diff;
  const inline = variant === 'inline';
  return <section className={cn('min-w-0 overflow-hidden bg-timeline-card', inline ? 'border-t border-border' : 'rounded-[10px] border border-border-strong')} aria-label="变更差异">
    {!inline && <TimelineCardHeader action={previewEnabled && <ChangePreviewButton target={target} />}>
      <div className="flex min-w-0 flex-1 items-center gap-2.5 pt-1.5">
        <span className="min-w-0 truncate font-mono text-[11px] text-text-500">{diff?.filename}{target.part === 'spec' && target.display_name ? ` · ${target.display_name}` : ''}</span>
        {(diff?.kind === 'outline' || diff?.kind === 'fields' || diff?.kind === 'text') && <ChangeStats insertions={target.insertions} deletions={target.deletions} />}
      </div>
    </TimelineCardHeader>}
    {!diff || diff.kind === 'unavailable' ? <p role="status" className="px-3.5 py-3 text-xs text-text-600">{diff?.error ?? '变更差异不可用。'}</p>
      : diff.kind === 'binary' ? <p className="px-3.5 py-3 text-xs text-text-600">{diff.status === 'added' ? '已新增文件' : diff.status === 'deleted' ? '已删除文件' : '已修改文件'}</p>
      : <LongContent horizontalScroll hideScrollbar maxHeight={210}
        className={inline ? 'py-2' : undefined}
        contentClassName={cn('w-max min-w-full', !inline && 'pt-2.5 pb-3.5')}
        controlsPlacement={inline ? 'below' : 'overlay'} expandLabel={inline ? '展开全部' : '展开'}
        controlsClassName={inline ? 'mt-0 pt-1' : 'mt-0 pb-3'}
        fadeClassName="from-timeline-card/0 via-timeline-card/90 to-timeline-card"
        buttonClassName={cn('h-7 bg-timeline-card text-[11px] shadow-none', inline && 'rounded border-0 px-2.5 font-normal [&>svg]:hidden')}>
        {diff.kind === 'outline' ? <OutlineDiff groups={diff.groups ?? []} /> : diff.kind === 'fields' ? orderedFields(diff.fields ?? [], target.part).map((field, index) => <section key={`${field.field}:${index}`} className={index > 0 ? 'mt-3 border-t border-border pt-3' : ''}>
          <h3 className="sticky left-3.5 mb-1.5 w-max px-3.5 text-xs font-medium leading-5 text-text-700">{fieldLabel(field, target.part)}</h3>
          {field.rows.map((row, i) => <div key={i} className={`grid min-h-[25px] grid-cols-[24px_max-content] whitespace-pre px-3.5 py-0.5 text-xs leading-[21px] ${rowClass(row.kind)}`}>
            <span aria-label={row.kind === 'added' ? '新增' : '删除'} className="select-none font-mono">{row.kind === 'added' ? '+' : '−'}</span><span>{fieldValue(field, row.value)}</span>
          </div>)}
        </section>) : diff.hunks?.map((hunk, index) => <TextDiffHunk key={index} hunk={hunk} separated={index > 0} />)}
      </LongContent>}
    {footer && <footer className="border-t border-border px-3.5 py-3">{footer}</footer>}
  </section>;
}
