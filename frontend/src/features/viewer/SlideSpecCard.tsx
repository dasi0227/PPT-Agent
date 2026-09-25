import { Eye, MessageSquareText } from 'lucide-react';
import type { SlideRole, SlideSpec } from '../../api/types';
import { ManagementEditor } from './ManagementEditor';
import { SpecFields } from './AuthoringFields';
import { elementTypeLabel, slideRoleLabel } from './semanticLabels';

export function SlideSpecCard({ title, spec, role, projectId, slideId, hash, sceneRevision, blocked }: {
  projectId?: string; slideId?: string; hash?: string; sceneRevision?: number; blocked?: string;
  title: string;
  spec?: SlideSpec;
  role: SlideRole;
}) {
  const details = spec ? <SpecDetails spec={spec} /> : <p className="text-sm text-text-600">设计稿尚未生成，可填写核心信息和内容元素，或交给 Agent 创建。</p>;
  return (
    <article className="scrollbar-none h-full w-full overflow-y-auto rounded-xl border border-border bg-surface p-7 shadow-sm">
      <div className="flex items-center justify-between gap-2">
        <span className="rounded bg-accent-soft px-2 py-1 text-[10px] font-semibold text-accent">
          {slideRoleLabel(role)}
        </span>
      </div>
      <h2 className="mt-5 text-2xl font-semibold text-text-900">{title}</h2>
      <div className="mt-5">
        {projectId && slideId ? <ManagementEditor projectId={projectId} value={spec ?? { key_message: '', elements: [] }} hash={hash} sceneRevision={sceneRevision} blocked={blocked} create={!spec}
          mutation={value => ({ op: 'slide.spec.write', slide_id: slideId, spec: value })}
          fields={(value, onChange) => <SpecFields value={value} onChange={onChange} />}>
          {details}
        </ManagementEditor> : details}
      </div>
    </article>
  );
}

function SpecDetails({ spec }: { spec: SlideSpec }) {
  return <>
      <div className="mt-4 flex gap-2 rounded-lg bg-accent-soft/70 p-3 text-base text-text-900">
        <MessageSquareText className="h-4 w-4 shrink-0 text-accent" />
        <strong>{spec.key_message}</strong>
      </div>
      <div className="mt-5 space-y-3 border-t border-border pt-4">
        {spec.layout && <p className="text-xs text-text-600">布局建议：{spec.layout}</p>}
        {spec.elements.map((element, index) => (
          <div key={`${element.type}-${index}`} className="flex gap-2 text-sm text-text-600">
            <Eye className="mt-0.5 h-4 w-4 shrink-0 text-accent" />
            <div>
              <strong className="text-xs">{elementTypeLabel(element.type)}</strong>
              <p className="mt-1">{element.intent}</p>
            </div>
          </div>
        ))}
      </div>
    </>;
}
