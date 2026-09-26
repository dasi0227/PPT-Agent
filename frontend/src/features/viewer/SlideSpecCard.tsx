import { ClipboardCheck } from 'lucide-react';
import type { SlideSpec } from '../../api/types';
import { Button, Disclosure, InlineNotice } from '../../components/ui/primitives';
import { SpecFields } from './AuthoringFields';
import { changedFields, ManagementEditor } from './ManagementEditor';
import { partLabel } from './semanticLabels';

const emptySpec: SlideSpec = { key_message: '', elements: [] };

export function SlideSpecCard({ title, spec, projectId, slideId, hash, sceneRevision, blocked, error, onRetry }: {
  projectId?: string; slideId?: string; hash?: string; sceneRevision?: number; blocked?: string;
  error?: string; onRetry?: () => void; title: string; spec?: SlideSpec;
}) {
  const notice = error ? <InlineNotice tone="danger" className="mb-6 flex flex-wrap items-center justify-between gap-3">
    <div><span>{spec ? '规格要求更新失败，当前显示上次加载的内容。' : '规格要求加载失败，请重试。'}</span>
      <Disclosure label="错误详情"><p className="break-all font-mono text-[11px]">{error}</p></Disclosure>
    </div>{onRetry && <Button variant="secondary" onClick={onRetry}>重试</Button>}
  </InlineNotice> : undefined;
  return <ManagementEditor key={`${projectId}:${slideId}`} projectId={slideId ? projectId : undefined} resourceKey={`spec:${slideId}`}
    title={partLabel('spec')} icon={ClipboardCheck} value={spec ?? emptySpec} hash={hash} sceneRevision={sceneRevision}
    blocked={error ? '加载失败，请重试后编辑。' : blocked} notice={notice} jsonAvailable={Boolean(spec)}
    jsonValue={spec && slideId ? { [slideId]: spec } : spec}
    mutation={(next, previous) => spec
      ? { op: 'slide.spec.patch', slide_id: slideId!, patch: changedFields(previous, next) }
      : { op: 'slide.spec.write', slide_id: slideId!, spec: next }}>
    {editor => <SpecFields editor={editor} title={title} creating={!spec} />}
  </ManagementEditor>;
}
