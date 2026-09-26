import { Fragment } from 'react';
import { Copy } from 'lucide-react';
import { Button } from '../../components/ui/primitives';
import { showGlobalError, showGlobalSuccess } from '../../stores/toastStore';

function highlight(line: string) {
  const tokens = /"(?:\\.|[^"\\])*"\s*:|"(?:\\.|[^"\\])*"|\b(?:true|false|null|-?\d+(?:\.\d+)?)\b/g;
  const parts = [];
  let cursor = 0;
  for (const match of line.matchAll(tokens)) {
    parts.push(<Fragment key={`plain-${cursor}`}>{line.slice(cursor, match.index)}</Fragment>);
    const kind = match[0].endsWith(':') ? 'key' : match[0].startsWith('"') ? 'string' : 'literal';
    parts.push(<span key={`token-${match.index}`} className={`management-json-${kind}`}>{match[0]}</span>);
    cursor = match.index! + match[0].length;
  }
  parts.push(<Fragment key="tail">{line.slice(cursor)}</Fragment>);
  return parts;
}

export function JSONPreview({ value }: { value: unknown }) {
  const text = JSON.stringify(value, null, 2) ?? '';
  return <section className="management-json" aria-label="JSON 只读预览">
    <header><span>只读预览</span><Button variant="ghost" onClick={async () => {
      try { await navigator.clipboard.writeText(text); showGlobalSuccess('JSON 已复制'); }
      catch { showGlobalError('复制失败，请选中内容后手动复制。'); }
    }}><Copy className="h-3.5 w-3.5" aria-hidden="true" />复制</Button></header>
    <div className="management-json-scroll" tabIndex={0} aria-label="JSON 内容">
      <pre><code>{text.split('\n').map((line, index) => <span className="management-json-line" key={index}>
        <span className="management-json-number" aria-hidden="true">{index + 1}</span><span>{highlight(line)}{'\n'}</span>
      </span>)}</code></pre>
    </div>
  </section>;
}
