import { FileText, Palette } from 'lucide-react';
import type { ProjectContentSnapshot } from '../../api/types';
import { Button, InlineNotice, Skeleton } from '../../components/ui/primitives';
import type { ProjectDocument } from '../../stores/deckStore';
import { DocumentCanvas } from './DocumentCanvas';
import { changedFields, ManagementEditor } from './ManagementEditor';
import { ManifestFields, DesignFields } from './AuthoringFields';
import { partLabel } from './semanticLabels';

export function ProjectDocumentView({ document, snapshot, error, onRetry, blocked }: {
  blocked?: string; document: ProjectDocument; snapshot?: ProjectContentSnapshot; error?: string; onRetry: () => void;
}) {
  const title = partLabel(document);
  const icon = document === 'manifest' ? FileText : Palette;
  const notice = error ? <InlineNotice tone="danger" className="mb-6 flex flex-wrap items-center justify-between gap-3">
    <span>{snapshot ? '内容更新失败，当前显示上次加载的内容。' : '内容加载失败，请重试。'}</span>
    <Button variant="secondary" onClick={onRetry}>重试</Button>
  </InlineNotice> : undefined;
  if (!snapshot) return <DocumentCanvas title={title} icon={icon}>
    {notice ?? <div className="space-y-4" role="status" aria-label="正在加载项目文档">
      <Skeleton className="h-5 w-1/3" /><Skeleton className="h-4 w-full" /><Skeleton className="h-4 w-4/5" />
    </div>}
  </DocumentCanvas>;
  const common = { projectId: snapshot.project_id, sceneRevision: snapshot.scene_revision, title, icon, notice,
    blocked: error ? '加载失败，请重试后编辑。' : blocked };
  return document === 'manifest' ? <ManagementEditor {...common} key={`${snapshot.project_id}:manifest`} resourceKey="manifest" value={snapshot.manifest} hash={snapshot.hashes.manifest}
    mutation={(next, previous) => ({ op: 'manifest.patch', patch: changedFields(previous, next) })}>
    {editor => <ManifestFields editor={editor} />}
  </ManagementEditor> : <ManagementEditor {...common} key={`${snapshot.project_id}:design`} resourceKey="design" value={snapshot.design} hash={snapshot.hashes.design}
    mutation={(next, previous) => ({ op: 'design.patch', patch: changedFields(previous, next) })}>
    {editor => <DesignFields editor={editor} />}
  </ManagementEditor>;
}
