import type { Manifest, ProjectContentSnapshot } from '../../api/types';
import { Button, InlineNotice, Skeleton } from '../../components/ui/primitives';
import type { ProjectDocument } from '../../stores/deckStore';
import { DesignDetails } from './DesignSummary';
import { DocumentSection } from './DocumentSection';
import { partLabel } from './semanticLabels';

function TextList({ items, empty }: { items: string[]; empty: string }) {
  const values = items.map((item) => item.trim()).filter(Boolean);
  return values.length ? (
    <ul className="list-disc space-y-2 pl-5 marker:text-text-400">
      {values.map((item, index) => <li key={index} className="whitespace-pre-wrap break-words pl-1">{item}</li>)}
    </ul>
  ) : <p className="text-text-600">{empty}</p>;
}

function languageLabel(language: string) {
  const labels: Record<string, string> = {
    zh: '中文', 'zh-cn': '简体中文', 'zh-hans': '简体中文',
    'zh-tw': '繁体中文', 'zh-hk': '繁体中文', 'zh-hant': '繁体中文',
    en: '英语', 'en-us': '英语（美国）', 'en-gb': '英语（英国）',
    ja: '日语', ko: '韩语', fr: '法语', de: '德语', es: '西班牙语',
  };
  return labels[language.trim().toLowerCase()] ?? language.trim();
}

function ManifestDetails({ manifest }: { manifest: Manifest }) {
  return (
    <div className="space-y-7 text-sm font-normal leading-6 text-text-700">
      {[
        { label: '演示标题', value: manifest.title.trim(), empty: '未命名演示' },
        { label: '演示语言', value: languageLabel(manifest.language), empty: '待明确' },
        { label: '演示目标', value: manifest.goal.trim(), empty: '待明确' },
        { label: '目标受众', value: manifest.audience.trim(), empty: '待明确' },
      ].map(({ label, value, empty }) => (
        <DocumentSection key={label} title={label}>
          <p className={`whitespace-pre-wrap break-words ${value ? '' : 'text-text-600'}`}>{value || empty}</p>
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
    <div className="scrollbar-none min-h-0 min-w-0 flex-1 overflow-y-auto bg-canvas p-[clamp(1rem,3%,2rem)]" role="region" aria-label={partLabel(document)} tabIndex={0}>
      <article className="mx-auto w-full max-w-[800px] rounded-lg border border-border bg-surface p-[clamp(1.25rem,4%,2.5rem)] shadow-[0_2px_8px_rgba(71,85,105,0.06)]">
        <header className="mb-7 border-b border-border pb-6">
          <h1 className="text-2xl font-semibold tracking-tight text-text-900">{partLabel(document)}</h1>
        </header>
        {error && (
          <InlineNotice tone="danger" className="mb-6 flex flex-wrap items-center justify-between gap-3">
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
