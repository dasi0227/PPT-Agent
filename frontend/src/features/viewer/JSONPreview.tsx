import { Fragment, type ReactNode } from 'react';
import { SourceCanvas } from './SourceCanvas';

function highlight(line: string) {
  const tokens = /"(?:\\.|[^"\\])*"\s*:|"(?:\\.|[^"\\])*"|\b(?:true|false|null|-?\d+(?:\.\d+)?)\b/g;
  const parts = [];
  let cursor = 0;
  for (const match of line.matchAll(tokens)) {
    parts.push(<Fragment key={`plain-${cursor}`}>{line.slice(cursor, match.index)}</Fragment>);
    const kind = match[0].endsWith(':') ? 'key' : match[0].startsWith('"') ? 'string' : 'literal';
    parts.push(<span key={`token-${match.index}`} className={`source-json-${kind}`}>{match[0]}</span>);
    cursor = match.index! + match[0].length;
  }
  parts.push(<Fragment key="tail">{line.slice(cursor)}</Fragment>);
  return parts;
}

export function JSONPreview({ title, value, notice }: { title: string; value: unknown; notice?: ReactNode }) {
  const text = JSON.stringify(value, null, 2) ?? '';
  return <SourceCanvas title={title} language="JSON" label="JSON 只读预览" copyText={text} notice={notice}>
    <div className="source-json-scroll" tabIndex={0} aria-label="JSON 内容">
      <pre><code>{text.split('\n').map((line, index) => <span className="source-json-line" key={index}>
        <span className="source-json-number" aria-hidden="true">{index + 1}</span><span className="source-json-text">{highlight(line)}{'\n'}</span>
      </span>)}</code></pre>
    </div>
  </SourceCanvas>;
}
