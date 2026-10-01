import { useId, useState } from 'react';
import { ChevronUp, CornerUpLeft, Ellipsis, File, Folder } from 'lucide-react';
import type { DiffHunk, FieldDiff, PublicTarget, DecorationPlacement, DecorationType, TextDiffRow, OutlineDiffGroup } from '../../api/types';
import { useProjectStore } from '../../stores/projectStore';
import { orderedSlides } from '../deck/selectors';
import { decorationPlacementLabel, decorationTypeLabel, elementTypeLabel, slideRoleLabel } from '../viewer/semanticLabels';
import { LongContent } from './LongContent';
import { openSourceTarget } from './SourceCard';
import { TimelineCardHeader, timelineCardActionClass } from './TimelineCardHeader';
import { ChangeStats } from './ChangeStats';

const fieldLabels: Record<string, string> = {
  title: '演示标题', language: '内容语言', pages: '期望页数', audience: '受众', goal: '演示目标',
  requirements: '内容要求', prohibitions: '内容禁忌', key_message: '核心信息', layout: '布局建议',
  role: '页面角色', elements: '内容元素',
};
function fieldLabel(field: FieldDiff, part: PublicTarget['part']): string {
  if (field.label) return field.label;
  if (field.field.startsWith('decorations.')) return `${decorationTypeLabel(field.field.slice(12) as DecorationType)}位置`;
  if (field.field === 'requirements' && part === 'design') return '设计需求';
  return fieldLabels[field.field] ?? (field.field || '内容');
}
function fieldValue(field: FieldDiff, source: string): string {
  const value: unknown = JSON.parse(source);
  if (field.field === 'role' && typeof value === 'string') return slideRoleLabel(value);
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

export function ChangeDiffCard({ target }: { target: PublicTarget }) {
  const projectId = useProjectStore(state => state.activeProjectId);
  const snapshot = useProjectStore(state => projectId ? state.contentByProjectId[projectId] : undefined);
  const exists = !!snapshot && (target.type === 'deck' || orderedSlides(snapshot).some(slide => slide.id === target.slide_id));
  const diff = target.diff;
  const canPreview = !!projectId && target.type !== 'file' && exists && diff?.status !== 'deleted';
  return <section className="min-w-0 overflow-hidden rounded-[10px] border border-border-strong bg-timeline-card" aria-label="变更差异">
    <TimelineCardHeader action={target.type !== 'file' && <button type="button" disabled={!canPreview} className={timelineCardActionClass} onClick={() => openSourceTarget(target)}>
      <CornerUpLeft className="h-3.5 w-3.5" strokeWidth={1.75} aria-hidden="true" /><span>预览</span>
    </button>}>
      <div className="flex min-w-0 flex-1 items-center gap-2.5 pt-1.5">
        <span className="min-w-0 truncate font-mono text-[11px] text-text-500">{diff?.filename}{target.part === 'spec' && target.display_name ? ` · ${target.display_name}` : ''}</span>
        {(diff?.kind === 'outline' || diff?.kind === 'fields' || diff?.kind === 'text') && <ChangeStats insertions={target.insertions} deletions={target.deletions} />}
      </div>
    </TimelineCardHeader>
    {!diff || diff.kind === 'unavailable' ? <p role="status" className="px-3.5 py-3 text-xs text-text-600">{diff?.error ?? '变更差异不可用。'}</p>
      : diff.kind === 'binary' ? <p className="px-3.5 py-3 text-xs text-text-600">{diff.status === 'added' ? '已新增文件' : diff.status === 'deleted' ? '已删除文件' : '已修改文件'}</p>
      : <LongContent horizontalScroll hideScrollbar maxHeight={210} contentClassName="w-max min-w-full pt-2.5 pb-3.5" controlsClassName="mt-0 pb-3" fadeClassName="from-timeline-card/0 via-timeline-card/90 to-timeline-card" buttonClassName="h-7 bg-timeline-card text-[11px] shadow-none">
        {diff.kind === 'outline' ? <OutlineDiff groups={diff.groups ?? []} /> : diff.kind === 'fields' ? diff.fields?.map((field, index) => <section key={`${field.field}:${index}`} className={index > 0 ? 'mt-3 border-t border-border pt-3' : ''}>
          <h3 className="sticky left-3.5 mb-1.5 w-max px-3.5 text-xs font-medium leading-5 text-text-700">{fieldLabel(field, target.part)}</h3>
          {field.rows.map((row, i) => <div key={i} className={`grid min-h-[25px] grid-cols-[24px_max-content] whitespace-pre px-3.5 py-0.5 text-xs leading-[21px] ${rowClass(row.kind)}`}>
            <span aria-label={row.kind === 'added' ? '新增' : '删除'} className="select-none font-mono">{row.kind === 'added' ? '+' : '−'}</span><span>{fieldValue(field, row.value)}</span>
          </div>)}
        </section>) : diff.hunks?.map((hunk, index) => <TextDiffHunk key={index} hunk={hunk} separated={index > 0} />)}
      </LongContent>}
  </section>;
}
