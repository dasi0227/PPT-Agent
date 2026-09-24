export class SourceFormatError extends Error {
  constructor(message: string, public readonly from?: number, public readonly to?: number) { super(message); }
}

export async function formatStrictJSON(text: string): Promise<string> {
  // JSON.parse alone accepts duplicate keys, so inspect each object in the
  // source tree before converting it into a JavaScript value.
  let value: unknown;
  try { value = JSON.parse(text) as unknown; }
  catch (error) {
    const raw = error instanceof Error ? error.message : 'JSON 格式无效';
    const position = /position (\d+)/.exec(raw);
    const from = position ? Number(position[1]) : undefined;
    throw new SourceFormatError(raw, from, from === undefined ? undefined : Math.min(text.length, from + 1));
  }
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('源文件必须是 JSON 对象');
  const { jsonLanguage } = await import('@codemirror/lang-json');
  const tree = jsonLanguage.parser.parse(text);
  tree.iterate({ enter(node) {
    if (node.name !== 'Object') return;
    const keys = new Set<string>();
    for (const property of node.node.getChildren('Property')) {
      const name = property.getChild('PropertyName');
      if (!name) continue;
      const key = JSON.parse(text.slice(name.from, name.to)) as string;
      if (keys.has(key)) throw new SourceFormatError(`JSON 字段重复：${key}`, name.from, name.to);
      keys.add(key);
    }
  } });
  return `${JSON.stringify(value, null, 2)}\n`;
}

let worker: Worker | null = null;
let requestId = 0;
const requests = new Map<number, { resolve: (value: string) => void; reject: (reason: Error) => void }>();

export function formatHTML(text: string): Promise<string> {
  if (!worker) {
    worker = new Worker(new URL('../features/viewer/sourceFormatter.worker.ts', import.meta.url), { type: 'module' });
    worker.onmessage = (event: MessageEvent<{ id: number; formatted?: string; error?: string }>) => {
      const pending = requests.get(event.data.id);
      if (!pending) return;
      requests.delete(event.data.id);
      if (event.data.error) pending.reject(new Error(event.data.error));
      else pending.resolve(event.data.formatted ?? '');
    };
    worker.onerror = () => {
      for (const pending of requests.values()) pending.reject(new Error('HTML 格式化器加载失败'));
      requests.clear();
      worker?.terminate(); worker = null;
    };
  }
  return new Promise((resolve, reject) => {
    const id = ++requestId;
    requests.set(id, { resolve, reject });
    worker!.postMessage({ id, text });
  });
}
