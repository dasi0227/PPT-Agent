import type { TimelineItem } from './eventReducer';

// HistoryEntry 与后端 backend/internal/run/history_writer.go 的 schema 对齐（UX spec §2.3）。
export interface HistoryEntry {
  seq: number;
  ts: number;
  run_id: string;
  turn: 'user' | 'agent';
  type: 'user_turn' | 'markdown' | 'info' | 'needs_input' | 'final_result' | 'error' | 'tool_call' | 'thought' | 'tool_result' | 'artifact';
  data: Record<string, any>;
}

// 把 <thread>.jsonl 的每行还原成 TimelineItem；未知 type 丢弃，避免 UI 崩溃。
// 传入非数组（例如后端 500 fallback）时直接返回空数组，防止 replay 抛异常污染日志。
export function hydrateFromHistory(entries: HistoryEntry[] | unknown): TimelineItem[] {
  if (!Array.isArray(entries)) return [];
  return entries
    .slice()
    .sort((a, b) => a.seq - b.seq)
    .map(toTimelineItem)
    .filter((x): x is TimelineItem => x !== null);
}

function toTimelineItem(e: HistoryEntry): TimelineItem | null {
  const timestamp = (e.ts || 0) * 1000;
  const baseId = `hist_${e.seq}`;
  switch (e.type) {
    case 'user_turn':
      return { id: baseId, type: 'user_turn', text: String(e.data.text ?? ''), timestamp };
    case 'markdown':
    case 'info':
      return { id: baseId, type: 'markdown', text: String(e.data.text ?? ''), timestamp };
    case 'tool_call':
      return {
        id: baseId,
        type: 'tool_call',
        call_id: String(e.data.call_id ?? ''),
        tool: String(e.data.tool ?? ''),
        args: e.data.args ?? {},
        status: e.data.status ?? 'running',
        observation: e.data.observation,
        artifacts: Array.isArray(e.data.artifacts) ? e.data.artifacts : [],
        timestamp,
        hiddenFromTimeline: e.data.tool === 'finish'
      };
    case 'thought':
      return { id: baseId, type: 'thought', text: String(e.data.text ?? ''), timestamp };
    case 'needs_input':
      return {
        id: String(e.data.id ?? baseId),
        type: 'needs_input',
        prompt: String(e.data.prompt ?? ''),
        choices: e.data.choices,
        timestamp,
      };
    case 'final_result':
      return { id: baseId, type: 'final_result', result: e.data.result, timestamp };
    case 'error':
      return { id: baseId, type: 'error', code: e.data.code, message: String(e.data.message ?? ''), timestamp };
    default:
      return null;
  }
}
