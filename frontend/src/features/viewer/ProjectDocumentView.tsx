import { ContentRequirementsIcon } from '../../components/ui/ContentRequirementsIcon';
import { useState } from 'react';
import { Palette, List } from 'lucide-react';
import type { ProjectContentSnapshot } from '../../api/types';
import { Button, InlineNotice, Skeleton } from '../../components/ui/primitives';
import type { ProjectDocument } from '../../stores/deckStore';
import { DocumentCanvas } from './DocumentCanvas';
import { changedFields, ManagementEditor } from './ManagementEditor';
import { ManifestFields, DesignFields } from './AuthoringFields';
import { partLabel } from './semanticLabels';
import { useProjectStore } from '../../stores/projectStore';
import { useResourceApprovalStore } from '../../stores/resourceApprovalStore';
import { ApprovalDraftEditor } from './ApprovalDraftEditor';
import { OutlineDocumentEditor } from './OutlineDocumentEditor';

function MissingDocument({ snapshot, document, blocked }: { snapshot: ProjectContentSnapshot; document: 'manifest' | 'design'; blocked?: string }) {
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState('');
  const title = partLabel(document);
  const icon = document === 'manifest' ? ContentRequirementsIcon : Palette;
  const create = async () => {
    if (creating || blocked) return;
    setCreating(true);
    setCreateError('');
    try {
      await useProjectStore.getState().mutateProject(snapshot.project_id, {
        op: document === 'manifest' ? 'manifest.create' : 'design.create',
        expected_scene_revision: snapshot.scene_revision,
      });
    } catch (error) {
      await useProjectStore.getState().loadProjectContent(snapshot.project_id);
      setCreateError(error instanceof Error ? error.message : '新建失败');
    } finally {
      setCreating(false);
    }
  };
  return <DocumentCanvas title={title} icon={icon}>
    <div className="flex min-h-48 flex-col items-center justify-center gap-4">
      <p className="text-sm text-text-600">暂无文件</p>
      <Button variant="primary" disabled={Boolean(blocked) || creating} onClick={() => void create()}>{creating ? '新建中…' : '新建'}</Button>
      {createError && <p className="text-xs text-danger" role="alert">{createError}</p>}
    </div>
  </DocumentCanvas>;
}

export function ProjectDocumentView({ document, snapshot, error, onRetry, blocked }: {
  blocked?: string; document: ProjectDocument; snapshot?: ProjectContentSnapshot; error?: string; onRetry: () => void;
}) {
  const activeApproval = useResourceApprovalStore(state => state.active);
  const activeProjectId = useProjectStore(state => state.activeProjectId);
  if (activeApproval?.resource === document && activeApproval.projectId === activeProjectId) return <ApprovalDraftEditor active={activeApproval} />;
  const title = partLabel(document);
  const icon = document === 'manifest' ? ContentRequirementsIcon : document === 'outline' ? List : Palette;
  const notice = error ? <InlineNotice tone="danger" className="mb-6 flex flex-wrap items-center justify-between gap-3">
    <span>{snapshot ? '内容更新失败，当前显示上次加载的内容。' : '内容加载失败，请重试。'}</span>
    <Button variant="secondary" onClick={onRetry}>重试</Button>
  </InlineNotice> : undefined;
  if (!snapshot) return <DocumentCanvas title={title} icon={icon}>
    {notice ?? <div className="space-y-4" role="status" aria-label="正在加载项目文档">
      <Skeleton className="h-5 w-1/3" /><Skeleton className="h-4 w-full" /><Skeleton className="h-4 w-4/5" />
    </div>}
  </DocumentCanvas>;
  if (document === 'outline') return <OutlineDocumentEditor snapshot={snapshot} notice={notice}
    blocked={error ? '加载失败，请重试后编辑。' : blocked} />;
  const common = { projectId: snapshot.project_id, sceneRevision: snapshot.scene_revision, title, icon, notice,
    blocked: error ? '加载失败，请重试后编辑。' : blocked };
  if (document === 'manifest') {
    if (!snapshot.manifest) return <MissingDocument snapshot={snapshot} document="manifest" blocked={blocked} />;
    return <ManagementEditor {...common} key={`${snapshot.project_id}:manifest`} resourceKey="manifest" value={snapshot.manifest} hash={snapshot.hashes.manifest}
      mutation={(next, previous) => ({ op: 'manifest.patch', patch: changedFields(previous, next) })}>
      {editor => <ManifestFields editor={editor} />}
    </ManagementEditor>;
  }
  if (!snapshot.design) return <MissingDocument snapshot={snapshot} document="design" blocked={blocked} />;
  return <ManagementEditor {...common} key={`${snapshot.project_id}:design`} resourceKey="design" value={snapshot.design} hash={snapshot.hashes.design}
    mutation={(next, previous) => ({ op: 'design.patch', patch: changedFields(previous, next) })}>
    {editor => <DesignFields editor={editor} />}
  </ManagementEditor>;
}
