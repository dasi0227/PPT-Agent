import type { SlideSpec } from '../../api/types';
import { Button, Disclosure, InlineNotice } from '../../components/ui/primitives';
import { SpecFields } from './AuthoringFields';
import { DocumentCanvas } from './DocumentCanvas';
import { DocumentSection } from './DocumentSection';
import { ManagementEditor } from './ManagementEditor';
import { elementTypeLabel, partLabel, slideRoleLabel } from './semanticLabels';

export function SlideSpecCard({ title, spec, projectId, slideId, hash, sceneRevision, blocked, error, onRetry }: {
  projectId?: string; slideId?: string; hash?: string; sceneRevision?: number; blocked?: string;
  error?: string; onRetry?: () => void;
  title: string;
  spec?: SlideSpec;
}) {
  const details = (
    <div className="space-y-7 text-sm font-normal leading-6 text-text-700">
      <DocumentSection title="页面标题"><p className="whitespace-pre-wrap break-words">{title}</p></DocumentSection>
      <DocumentSection title="页面角色"><p>{slideRoleLabel(spec?.role)}</p></DocumentSection>
      {spec ? <SpecDetails spec={spec} /> : <p className="text-text-600">设计稿尚未生成，可填写核心信息和内容元素，或交给 Agent 创建。</p>}
    </div>
  );
  return (
    <DocumentCanvas title={partLabel('spec')}>
      {error && (
        <InlineNotice tone="danger" className="mb-6 flex flex-wrap items-center justify-between gap-3">
          <div>
            <span>{spec ? '设计稿更新失败，当前显示上次加载的内容。' : '设计稿加载失败，请重试。'}</span>
            <Disclosure label="错误详情"><p className="break-all font-mono text-[11px]">{error}</p></Disclosure>
          </div>
          {onRetry && <Button variant="secondary" onClick={onRetry}>重试</Button>}
        </InlineNotice>
      )}
      {projectId && slideId ? (
        <ManagementEditor projectId={projectId} value={spec ?? { key_message: '', elements: [] }} hash={hash} sceneRevision={sceneRevision} blocked={error ? '加载失败，请重试后编辑。' : blocked} create={!spec}
          mutation={value => ({ op: 'slide.spec.write', slide_id: slideId, spec: value })}
          fields={(value, onChange) => <SpecFields value={value} onChange={onChange} />}>
          {details}
        </ManagementEditor>
      ) : details}
    </DocumentCanvas>
  );
}

function SpecDetails({ spec }: { spec: SlideSpec }) {
  const keyMessage = spec.key_message.trim();
  const layout = spec.layout?.trim();
  return (
    <div className="space-y-7">
      <DocumentSection title="核心信息">
        <p className={`whitespace-pre-wrap break-words ${keyMessage ? '' : 'text-text-600'}`}>{keyMessage || '待明确'}</p>
      </DocumentSection>
      <DocumentSection title="布局建议">
        <p className={`whitespace-pre-wrap break-words ${layout ? '' : 'text-text-600'}`}>{layout || '暂无布局建议'}</p>
      </DocumentSection>
      <DocumentSection title="内容元素">
        {spec.elements.length ? (
          <ul className="space-y-4">
            {spec.elements.map((element, index) => (
              <li key={`${element.type}-${index}`}>
                <p className="font-medium text-text-900">{elementTypeLabel(element.type)}</p>
                <p className="whitespace-pre-wrap break-words">{element.intent}</p>
              </li>
            ))}
          </ul>
        ) : <p className="text-text-600">暂无内容元素</p>}
      </DocumentSection>
    </div>
  );
}
