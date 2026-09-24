import type { ReactNode } from 'react';
import type { Manifest, ProjectContentSnapshot } from '../../api/types';
import { Button, InlineNotice, Skeleton } from '../../components/ui/primitives';
import type { ProjectDocument } from '../../stores/deckStore';
import { DesignDetails } from './DesignSummary';
import { partLabel } from './semanticLabels';

function DocumentSection({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section>
      <h2 className="mb-2 text-lg font-semibold leading-7 text-text-900">{title}</h2>
      {children}
    </section>
  );
}

function TextList({ items, empty }: { items: string[]; empty: string }) {
  const values = items.map((item) => item.trim()).filter(Boolean);
  return values.length ? (
    <ul className="list-disc space-y-2 pl-5 marker:text-text-400">
      {values.map((item, index) => <li key={index} className="whitespace-pre-wrap break-words pl-1">{item}</li>)}
    </ul>
  ) : <p className="text-text-400">{empty}</p>;
}

function languageLabel(language: string) {
  const labels: Record<string, string> = {
    zh: '中文', 'zh-cn': '简体中文', 'zh-hans': '简体中文',
    'zh-tw': '繁体中文', 'zh-hk': '繁体中文', 'zh-hant': '繁体中文',
    en: '英语', 'en-us': '英语（美国）', 'en-gb': '英语（英国）',
    ja: '日语', ko: '韩语', fr: '法语', de: '德语', es: '西班牙语',
  };
  return labels[language.trim().toLowerCase()] ?? (language.trim() || '待明确');
}

function ManifestDetails({ manifest }: { manifest: Manifest }) {
  return (
    <div className="space-y-7 text-sm leading-7 text-text-700">
      {[
        ['演示标题', manifest.title.trim() || '未命名演示'],
        ['演示目标', manifest.goal.trim() || '待明确'],
        ['目标受众', manifest.audience.trim() || '待明确'],
        ['演示语言', languageLabel(manifest.language)],
      ].map(([label, value]) => (
        <DocumentSection key={label} title={label}>
          <p className="whitespace-pre-wrap break-words">{value}</p>
        </DocumentSection>
      ))}
      <DocumentSection title="内容要求">
        <TextList items={manifest.requirements} empty="暂无额外要求" />
      </DocumentSection>
      <DocumentSection title="限制与禁忌">
        <TextList items={manifest.prohibitions} empty="暂无额外限制" />
      </DocumentSection>
    </div>
  );
}

export function ProjectDocumentView({ document, snapshot, error, onRetry }: {
  document: ProjectDocument;
  snapshot?: ProjectContentSnapshot;
  error?: string;
  onRetry: () => void;
}) {
  return (
    <div className="scrollbar-none min-h-0 flex-1 overflow-y-auto bg-surface" role="region" aria-label={partLabel(document)} tabIndex={0}>
      <article className="mx-auto w-full max-w-3xl px-6 py-8 sm:px-10 sm:py-10">
        <header className="mb-8">
          <h1 className="text-2xl font-semibold tracking-tight text-text-900">{partLabel(document)}</h1>
        </header>
        {error && (
          <InlineNotice tone="danger" className="mb-6 flex items-center justify-between gap-3">
            <span>{snapshot ? '内容更新失败，当前显示上次加载的内容。' : '内容加载失败，请重试。'}</span>
            <Button variant="secondary" onClick={onRetry}>重试</Button>
          </InlineNotice>
        )}
        {snapshot ? document === 'manifest' ? (
          <ManifestDetails manifest={snapshot.manifest} />
        ) : (
          <DesignDetails design={snapshot.design} />
        ) : !error ? (
          <div className="space-y-4" role="status" aria-label="正在加载项目文档">
            <Skeleton className="h-5 w-1/3" />
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-4 w-4/5" />
          </div>
        ) : null}
      </article>
    </div>
  );
}
