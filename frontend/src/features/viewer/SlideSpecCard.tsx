import { useEffect, useRef, useState, type ReactNode } from 'react';
import { ClipboardCheck } from 'lucide-react';
import type { SlidePurpose, SlideSpec } from '../../api/types';
import { Button, Disclosure, InlineNotice } from '../../components/ui/primitives';
import { Select } from '../../components/ui/select';
import { TextField } from '../../components/ui/text-field';
import { useDeckStore } from '../../stores/deckStore';
import { useProjectStore } from '../../stores/projectStore';
import { SpecFields } from './AuthoringFields';
import { DocumentCanvas } from './DocumentCanvas';
import { changedFields, ManagementEditor } from './ManagementEditor';
import { ManagementSection } from './ManagementFields';
import { partLabel, slidePurposeOptions } from './semanticLabels';

export function SlideSpecCard({ spec, projectId, slideId, hash, sceneRevision, blocked, error, onRetry }: {
  projectId?: string; slideId?: string; hash?: string; sceneRevision?: string; blocked?: string;
  error?: string; onRetry?: () => void; spec?: SlideSpec;
}) {
  const notice = error ? <InlineNotice tone="danger" className="mb-6 flex flex-wrap items-center justify-between gap-3">
    <div><span>{spec ? '设计稿更新失败，当前显示上次加载的内容。' : '设计稿加载失败，请重试。'}</span>
      <Disclosure label="错误详情"><p className="break-all font-mono text-[11px]">{error}</p></Disclosure>
    </div>{onRetry && <Button variant="secondary" onClick={onRetry}>重试</Button>}
  </InlineNotice> : undefined;
  const unavailable = error ? '加载失败，请重试后编辑。' : blocked;
  if (!spec) return <SpecCreationForm key={`${projectId}:${slideId}`} projectId={projectId} slideId={slideId}
    hash={hash} sceneRevision={sceneRevision} blocked={unavailable} notice={notice} />;
  return <ManagementEditor key={`${projectId}:${slideId}`} projectId={slideId ? projectId : undefined} resourceKey={`spec:${slideId}`}
    title={partLabel('spec')} icon={ClipboardCheck} value={spec} hash={hash} sceneRevision={sceneRevision}
    blocked={unavailable} notice={notice}
    mutation={(next, previous) => ({ op: 'slide.spec.patch', slide_id: slideId!, patch: changedFields(previous, next) })}>
    {editor => <SpecFields editor={editor} />}
  </ManagementEditor>;
}

function SpecCreationForm({ projectId, slideId, hash, sceneRevision, blocked, notice }: {
  projectId?: string; slideId?: string; hash?: string; sceneRevision?: string; blocked?: string; notice?: ReactNode;
}) {
  const [core, setCore] = useState('');
  const [purpose, setPurpose] = useState<SlidePurpose | ''>('');
  const [layout, setLayout] = useState('');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string>();
  const pending = useRef(false);
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    useDeckStore.getState().setSourceBlocked(true);
    return () => { mounted.current = false; useDeckStore.getState().setSourceBlocked(false); };
  }, []);
  const disabled = !projectId || !slideId || Boolean(blocked) || saving;
  const valid = purpose !== '' && core.trim().length > 0 && [...core.trim()].length <= 500
    && layout.trim().length > 0 && [...layout.trim()].length <= 80;
  async function save() {
    if (disabled || !valid || !projectId || !slideId || !purpose || pending.current) return;
    pending.current = true; setSaving(true); setError(undefined);
    try {
      await useProjectStore.getState().mutateProject(projectId, {
        op: 'slide.spec.write', slide_id: slideId,
        spec: { core: core.trim(), purpose, layout: layout.trim(), elements: [] },
        expected_hash: hash, expected_scene_revision: sceneRevision,
      });
    } catch (cause) {
      if (mounted.current) setError(cause instanceof Error ? cause.message : '保存失败，请重试。');
      void useProjectStore.getState().loadProjectContent(projectId);
    } finally {
      pending.current = false;
      if (mounted.current) setSaving(false);
    }
  }
  return <DocumentCanvas title={partLabel('spec')} icon={ClipboardCheck}>
    {notice}
    <form onSubmit={event => { event.preventDefault(); void save(); }}>
      <fieldset disabled={disabled}>
        <ManagementSection title="页面信息">
          <div className="management-field"><div className="management-label">页面用途</div><div className="management-select-wrap">
            <Select aria-label="页面用途" value={purpose} options={slidePurposeOptions} required disabled={disabled}
              className="management-select bg-transparent" onValueChange={value => setPurpose(value as SlidePurpose)} />
          </div></div>
          <div className="management-field"><div className="management-label">核心信息</div>
            <TextField label="核心信息" hideLabel value={core} onChange={setCore} minLength={1} maxLength={500} multiline />
          </div>
          <div className="management-field"><div className="management-label">布局建议</div>
            <TextField label="布局建议" hideLabel value={layout} onChange={setLayout} minLength={1} maxLength={80} />
          </div>
        </ManagementSection>
      </fieldset>
      {(error || blocked) && <p className="management-error" role="alert">{blocked ?? error}</p>}
      <div className="management-editor-actions"><Button type="submit" variant="primary" disabled={disabled || !valid}>{saving ? '保存中…' : '保存'}</Button></div>
    </form>
  </DocumentCanvas>;
}
