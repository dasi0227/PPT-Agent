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
  const sorted = entries.slice().sort((a, b) => a.seq - b.seq);
  const items: TimelineItem[] = [];

  for (const e of sorted) {
    const timestamp = (e.ts || 0) * 1000;
    const baseId = `hist_${e.seq}`;

    switch (e.type) {
      case 'user_turn':
        items.push({
          id: baseId, type: 'user_turn', text: String(e.data.text ?? ''), timestamp,
          target: e.data.target, interaction: e.data.interaction,
        });
        break;
      case 'markdown':
      case 'info': {
        const text = String(e.data.text ?? '');
        const last = items[items.length - 1];
        if (last && last.type === 'markdown') {
          last.text += text;
        } else {
          items.push({ id: baseId, type: 'markdown', text, timestamp });
        }
        break;
      }
      case 'tool_call':
        items.push({
          id: baseId,
          type: 'tool_call',
          call_id: String(e.data.call_id ?? ''),
          tool: String(e.data.tool ?? ''),
          args: e.data.args ?? {},
          status: 'running',
          observation: undefined,
          artifacts: [],
          timestamp,
          hiddenFromTimeline: e.data.tool === 'finish'
        });
        break;
      case 'tool_result': {
        const tc = items.find(i => i.type === 'tool_call' && i.call_id === e.data.call_id);
        if (tc && tc.type === 'tool_call') {
          tc.status = e.data.ok ? 'success' : 'failed';
          tc.observation = e.data.observation;
        }
        break;
      }
      case 'artifact': {
        if (e.data.artifact_type === 'design_spec') {
          items.push({
            id: baseId,
            type: 'artifact',
            artifact_type: 'design_spec',
            ref: e.data.ref,
            delivery: 'intermediate',
            timestamp
          });
        } else {
          // Attach to last tool_call
          const lastTc = [...items].reverse().find(i => i.type === 'tool_call');
          if (lastTc && lastTc.type === 'tool_call') {
            lastTc.artifacts.push({
              id: baseId,
              type: 'artifact',
              artifact_type: e.data.artifact_type,
              ref: e.data.ref,
              page_index: e.data.page_index,
              delivery: 'intermediate',
              timestamp
            });
          } else {
            items.push({
              id: baseId,
              type: 'artifact',
              artifact_type: e.data.artifact_type,
              ref: e.data.ref,
              page_index: e.data.page_index,
              delivery: 'intermediate',
              timestamp
            });
          }
        }
        break;
      }
      case 'thought':
        items.push({ id: baseId, type: 'thought', text: String(e.data.text ?? ''), timestamp });
        break;
      case 'needs_input':
        items.push({
          id: String(e.data.id ?? baseId),
          type: 'needs_input',
          prompt: String(e.data.prompt ?? ''),
          choices: e.data.choices,
          timestamp,
        });
        break;
      case 'final_result':
        items.push({ id: baseId, type: 'final_result', result: e.data.result, timestamp });
        break;
      case 'error':
        items.push({ id: baseId, type: 'error', code: e.data.code, message: String(e.data.message ?? ''), timestamp });
        break;
    }
  }

  return items;
}
